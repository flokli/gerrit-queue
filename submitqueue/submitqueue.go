package submitqueue

import (
	"fmt"

	"github.com/tweag/gerrit-queue/gerrit"

	log "github.com/sirupsen/logrus"
)

// SubmitQueue contains a list of series, a gerrit connection, and some project configuration
type SubmitQueue struct {
	Series         []*Serie
	gerrit         gerrit.IClient
	ProjectName    string
	BranchName     string
	HEAD           string
	SubmitQueueTag string // the tag used to submit something to the submit queue
}

// MakeSubmitQueue builds a new submit queue
func MakeSubmitQueue(gerritClient gerrit.IClient, projectName string, branchName string, submitQueueTag string) SubmitQueue {
	return SubmitQueue{
		Series:         make([]*Serie, 0),
		gerrit:         gerritClient,
		ProjectName:    projectName,
		BranchName:     branchName,
		SubmitQueueTag: submitQueueTag,
	}
}

// LoadSeries fills .Series by searching changesets, and assembling them to Series.
func (s *SubmitQueue) LoadSeries() error {
	var queryString = fmt.Sprintf("status:open project:%s branch:%s", s.ProjectName, s.BranchName)
	log.Debugf("Running query %s", queryString)

	// Download changesets from gerrit
	changesets, err := s.gerrit.SearchChangesets(queryString)
	if err != nil {
		return err
	}

	// Assemble to series
	series, err := AssembleSeries(changesets)
	if err != nil {
		return err
	}

	// Sort by size
	s.Series = SortSeries(series)
	return nil
}

// UpdateHEAD updates the HEAD field with the commit ID of the current HEAD
func (s *SubmitQueue) UpdateHEAD() error {
	HEAD, err := s.gerrit.GetHEAD(s.ProjectName, s.BranchName)
	if err != nil {
		return err
	}
	s.HEAD = HEAD
	return nil
}

// TODO: clear submit queue tag if missing +1/+2?

// IsAutoSubmittable returns true if a given Serie has all the necessary flags set
// meaning it would be fine to rebase and/or submit it.
// This means, every changeset needs to:
// - have the s.SubmitQueueTag hashtag
// - be verified (+1 by CI)
// - be code reviewed (+2 by a human)
func (s *SubmitQueue) IsAutoSubmittable(serie *Serie) bool {
	return serie.FilterAllChangesets(func(c *gerrit.Changeset) bool {
		return c.HasTag(s.SubmitQueueTag) && c.IsVerified && c.IsCodeReviewed
	})
}

// DoSubmit submits changes that can be submitted,
// and updates `Series` to contain the remaining ones
// Also updates `HEAD`.
func (s *SubmitQueue) DoSubmit() error {
	var remainingSeries []*Serie

	for _, serie := range s.Series {
		serieParentCommitIDs, err := serie.GetParentCommitIDs()
		if err != nil {
			return err
		}
		// we can only submit series with a single parent commit (otherwise they're not rebased)
		if len(serieParentCommitIDs) != 1 {
			return fmt.Errorf("%s has more than one parent commit, skipping", serie.String())
		}

		// if serie is auto-submittable and rebased on top of current master…
		if s.IsAutoSubmittable(serie) && serieParentCommitIDs[0] == s.HEAD {
			// submit the last changeset of the series, which submits intermediate ones too
			_, err := s.gerrit.SubmitChangeset(serie.ChangeSets[len(serie.ChangeSets)-1])
			if err != nil {
				// this might fail, for various reasons:
				//  - developers could have updated the changeset meanwhile, clearing +1/+2 bits
				//  - master might have advanced, so this changeset isn't rebased on top of master
				// TODO: we currently bail out entirely, but should be fine on the
				// next loop. We might later want to improve the logic to be a bit more
				// smarter (like log and try with the next one)
				return err
			}
			// advance head to the leaf of the current serie for the next iteration
			newHead, err := serie.GetLeafCommitID()
			if err != nil {
				return err
			}
			s.HEAD = newHead
		} else {
			remainingSeries = append(remainingSeries, serie)
		}
	}

	s.Series = remainingSeries
	return nil
}

// DoRebase rebases the next auto-submittable series on top of current HEAD
// they are still ordered by series size
// After a DoRebase, consumers are supposed to fetch state again via LoadSeries,
// as things most likely have changed, and error handling during partially failed rebases
// is really tricky
func (s *SubmitQueue) DoRebase() error {
	if s.HEAD == "" {
		return fmt.Errorf("current HEAD is an empty string, bailing out")
	}
	for _, serie := range s.Series {
		logger := log.WithFields(log.Fields{
			"serie": serie,
		})
		if !s.IsAutoSubmittable(serie) {
			logger.Debug("skipping non-auto-submittable series")
			continue
		}

		logger.Infof("rebasing on top of %s", s.HEAD)
		_, err := s.RebaseSerie(serie, s.HEAD)
		if err != nil {
			// We skip trivial rebase errors instead of bailing out.
			// TODO: we might want to remove s.SubmitQueueTag from the changeset,
			// but even without doing it,
			// we're merly spanning, and won't get stuck in trying to rebase the same
			// changeset over and over again, as some other changeset will likely succeed
			// with rebasing and will be merged by DoSubmit.
			logger.Warnf("failure while rebasing, continuing with next one: %s", err)
			continue
		} else {
			logger.Info("success rebasing on top of %s", s.HEAD)
			break
		}
	}

	return nil
}

// Run starts the submit and rebase logic.
func (s *SubmitQueue) Run() error {
	//TODO: log decisions made and add to some ring buffer
	var err error

	commitID, err := s.gerrit.GetHEAD(s.ProjectName, s.BranchName)
	if err != nil {
		log.Errorf("Unable to retrieve HEAD of branch %s at project %s: %s", s.BranchName, s.ProjectName, err)
		return err
	}
	s.HEAD = commitID

	err = s.LoadSeries()
	if err != nil {
		return err
	}
	if len(s.Series) == 0 {
		// Nothing to do!
		log.Warn("Nothing to do here")
		return nil
	}
	err = s.DoSubmit()
	if err != nil {
		return err
	}
	err = s.DoRebase()
	if err != nil {
		return err
	}
	return nil
}

// RebaseSerie rebases a whole serie on top of a given ref
// TODO: only rebase a single changeset. we don't really want to join disconnected series, by rebasing them on top of each other.
func (s *SubmitQueue) RebaseSerie(serie *Serie, ref string) (*Serie, error) {
	newSeries := &Serie{
		ChangeSets: make([]*gerrit.Changeset, len(serie.ChangeSets)),
	}

	rebaseOnto := ref
	for _, changeset := range serie.ChangeSets {
		newChangeset, err := s.gerrit.RebaseChangeset(changeset, rebaseOnto)

		if err != nil {
			// uh-oh…
			// TODO: think about error handling
			// TODO: remove the submit queue tag if the rebase fails (but only then, not on other errors)
			return newSeries, err
		}
		newSeries.ChangeSets = append(newSeries.ChangeSets, newChangeset)

		// the next changeset should be rebased on top of the current commit
		rebaseOnto = newChangeset.CommitID
	}
	return newSeries, nil
}
