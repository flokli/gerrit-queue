package submitqueue

import (
	"fmt"

	"github.com/tweag/gerrit-queue/gerrit"

	log "github.com/sirupsen/logrus"
)

// SubmitQueueTag is the tag used to determine whether something
// should be considered by the submit queue or not
const SubmitQueueTag = "submit_me"

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

// LoadSeries fills Series by searching and filtering changesets, and assembling them to Series.
func (s *SubmitQueue) LoadSeries() error {
	// Normally, we'd like to use a queryString like
	// "status:open project:myproject branch:mybranch hashtag:submitQueueTag label:verified=+1 label:code-review=+2"
	// to avoid filtering client-side
	// Due to https://github.com/andygrunwald/go-gerrit/issues/71,
	// we need to do this on the client (filterChangesets)
	var queryString = fmt.Sprintf("status:open project:%s branch:%s", s.ProjectName, s.BranchName)
	log.Debugf("Running query %s", queryString)

	// Download changesets from gerrit
	changesets, err := s.gerrit.SearchChangesets(queryString)
	if err != nil {
		return err
	}
	// // Filter to contain the SubmitQueueTag
	// changesets = gerrit.FilterChangesets(changesets, func(c *gerrit.Changeset) bool {
	// 	return c.HasTag(SubmitQueueTag)
	// })
	// Filter to be code reviewed and verified
	changesets = gerrit.FilterChangesets(changesets, func(c *gerrit.Changeset) bool {
		return c.IsCodeReviewed && c.IsVerified
	})

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

// DoSubmit submits changes that can be submitted,
// and updates `Series` to contain the remaining ones
// Also updates `BranchCommitID`.
func (s *SubmitQueue) DoSubmit() error {
	var remainingSeries []*Serie

	for _, serie := range s.Series {
		serieParentCommitIDs, err := serie.GetParentCommitIDs()
		if err != nil {
			return err
		}
		// we can only submit series with a single parent commit (otherwise they're not rebased)
		if len(serieParentCommitIDs) != 1 {
			return fmt.Errorf("%s has more than one parent commit", serie.String())
		}
		// if serie is rebased on top of current master…
		if serieParentCommitIDs[0] == s.HEAD {
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

// DoRebase rebases all remaining series on top of each other
// they should still be ordered by series size
// TODO: this will produce a very large series on the next run, so we might want to preserve individual series over multiple runs
func (s *SubmitQueue) DoRebase() error {
	newSeries := make([]*Serie, len(s.Series))
	futureHEAD := s.HEAD
	for _, serie := range s.Series {
		//TODO: don't rebase everything, just pick a "good candidate"

		logger := log.WithFields(log.Fields{
			"serie": serie,
		})
		logger.Infof("rebasing %s on top of %s", serie, futureHEAD)
		newSerie, err := s.RebaseSerie(serie, futureHEAD)
		if err != nil {
			logger.Warnf("unable to rebase serie %s", err)
			// TODO: we want to skip on trivial rebase errors instead of bailing out.
			// skip means adding that serie as it is to newSeries, without advancing previousLeafCommitId

			// TODO: we also should talk about when to remove the submit-queue tag
			// just because we scheduled a conflicting submit plan, doesn't mean this is not submittable.
			// so just removing the submit-queue tag would be unfair
			return err
		}
		newSeries = append(newSeries, newSerie)

		// prepare for next iteration
		futureHEAD, err = newSerie.GetLeafCommitID()
		if err != nil {
			// This should never happen
			logger.Errorf("new serie shouldn't be empty: %s", newSerie)
			return err
		}

	}
	s.Series = newSeries
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
