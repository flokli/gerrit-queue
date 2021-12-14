package submitqueue

import (
	"fmt"
	"sync"

	"github.com/apex/log"

	"github.com/tweag/gerrit-queue/gerrit"
)

// Runner is a struct existing across the lifetime of a single run of the submit queue
// it contains a mutex to avoid being run multiple times.
// In fact, it even cancels runs while another one is still in progress.
// It contains a Gerrit object facilitating access, a log object, the configured submit queue tag
// and a `wipSerie` (only populated if waiting for a rebase)
type Runner struct {
	mut              sync.Mutex
	currentlyRunning bool
	wipSerie         *gerrit.Serie
	logger           *log.Logger
	gerrit           *gerrit.Client
	submitQueueTag   string // the tag used to submit something to the submit queue
}

// NewRunner creates a new Runner struct
func NewRunner(logger *log.Logger, gerrit *gerrit.Client, submitQueueTag string) *Runner {
	return &Runner{
		logger:         logger,
		gerrit:         gerrit,
		submitQueueTag: submitQueueTag,
	}
}

// isAutoSubmittable determines if something could be autosubmitted, potentially requiring a rebase
// for this, it needs to:
//  * have the auto-submit label
//  * have gerrit's 'submittable' field set to true
// it doesn't check if the series is rebased on HEAD
func (r *Runner) isAutoSubmittable(s *gerrit.Serie) bool {
	for _, c := range s.ChangeSets {
		if c.Submittable != true || !c.HasTag(r.submitQueueTag) {
			return false
		}
	}
	return true
}

// IsCurrentlyRunning returns true if the runner is currently running
func (r *Runner) IsCurrentlyRunning() bool {
	return r.currentlyRunning
}

// GetWIPSerie returns the current wipSerie, if any, nil otherwiese
// Acquires a lock, so check with IsCurrentlyRunning first
func (r *Runner) GetWIPSerie() *gerrit.Serie {
	r.mut.Lock()
	defer func() {
		r.mut.Unlock()
	}()
	return r.wipSerie
}

// Trigger gets triggered periodically
func (r *Runner) Trigger(fetchOnly bool) error {
	// TODO: If CI fails, remove the auto-submit labels => rules.pl
	// Only one trigger can run at the same time
	r.mut.Lock()
	if r.currentlyRunning {
		return fmt.Errorf("Already running, skipping")
	}
	r.currentlyRunning = true
	r.mut.Unlock()
	defer func() {
		r.mut.Lock()
		r.currentlyRunning = false
		r.mut.Unlock()
	}()

	// Prepare the work by creating a local cache of gerrit state
	err := r.gerrit.Refresh()
	if err != nil {
		return err
	}

	// early return if we only want to fetch
	if fetchOnly {
		return nil
	}

	if r.wipSerie != nil {
		// refresh wipSerie with how it looks like in gerrit now
		wipSerie := r.gerrit.FindSerie(func(s *gerrit.Serie) bool {
			// the new wipSerie needs to have the same number of changesets
			if len(r.wipSerie.ChangeSets) != len(s.ChangeSets) {
				return false
			}
			// … and the same ChangeIDs.
			for idx, c := range s.ChangeSets {
				if r.wipSerie.ChangeSets[idx].ChangeID != c.ChangeID {
					return false
				}
			}
			return true
		})
		if wipSerie == nil {
			r.logger.WithField("wipSerie", r.wipSerie).Warn("wipSerie has disappeared")
			r.wipSerie = nil
		} else {
			r.wipSerie = wipSerie
		}
	}

	for {
		// initialize logger
		r.logger.Info("Running")
		if r.wipSerie != nil {
			// if we have a wipSerie
			l := r.logger.WithField("wipSerie", r.wipSerie)
			l.Info("Checking wipSerie")

			// discard wipSerie not rebased on HEAD
			// we rebase them at the end of the loop, so this means master advanced without going through the submit queue
			if !r.gerrit.SerieIsRebasedOnHEAD(r.wipSerie) {
				l.Warnf("HEAD has moved to %v while still waiting for wipSerie, discarding it", r.gerrit.GetHEAD())
				r.wipSerie = nil
				continue
			}

			// we now need to check CI feedback:
			// wipSerie might have failed CI in the meantime
			for _, c := range r.wipSerie.ChangeSets {
				if c == nil {
					l.Error("BUG: changeset is nil")
					continue
				}
				if c.Verified < 0 {
					l.WithField("failingChangeset", c).Warnf("wipSerie failed CI in the meantime, discarding.")
					r.wipSerie = nil
					continue
				}
			}

			// it might still be waiting for CI
			for _, c := range r.wipSerie.ChangeSets {
				if c == nil {
					l.Error("BUG: changeset is nil")
					continue
				}
				if c.Verified == 0 {
					l.WithField("pendingChangeset", c).Warnf("still waiting for CI feedback in wipSerie, going back to sleep.")
					// break the loop, take a look at it at the next trigger.
					return nil
				}
			}

			// it might be autosubmittable
			if r.isAutoSubmittable(r.wipSerie) {
				l.Infof("submitting wipSerie")
				// if the WIP changeset is ready (auto submittable and rebased on HEAD), submit
				for _, changeset := range r.wipSerie.ChangeSets {
					_, err := r.gerrit.SubmitChangeset(changeset)
					if err != nil {
						l.WithField("changeset", changeset).Error("error submitting changeset")
						r.wipSerie = nil
						return err
					}
				}
				r.wipSerie = nil
			} else {
				l.Error("BUG: wipSerie is not autosubmittable")
				r.wipSerie = nil
			}
		}

		r.logger.Info("Looking for series ready to submit")
		// Find serie, that:
		//  * has the auto-submit label
		//  * has +2 review
		//  * has +1 CI
		//  * is rebased on master
		serie := r.gerrit.FindSerie(func(s *gerrit.Serie) bool {
			return r.isAutoSubmittable(s) && s.ChangeSets[0].ParentCommitIDs[0] == r.gerrit.GetHEAD()
		})
		if serie != nil {
			r.logger.WithField("serie", serie).Info("Found serie to submit without necessary rebase")
			r.wipSerie = serie
			continue
		}

		// Find serie, that:
		//  * has the auto-submit label
		//  * has +2 review
		//  * has +1 CI
		//  * is NOT rebased on master
		serie = r.gerrit.FindSerie(r.isAutoSubmittable)
		if serie == nil {
			r.logger.Info("no more submittable series found, going back to sleep.")
			break
		}

		l := r.logger.WithField("serie", serie)
		l.Info("found serie, which needs a rebase")
		// TODO: move into Client.RebaseSeries function
		head := r.gerrit.GetHEAD()
		for _, changeset := range serie.ChangeSets {
			changeset, err := r.gerrit.RebaseChangeset(changeset, head)
			if err != nil {
				l.Error(err.Error())
				return err
			}
			head = changeset.CommitID
		}
		// we don't need to care about updating the rebased changesets or getting the updated HEAD,
		// as we'll refetch it on the beginning of the next trigger anyways
		r.wipSerie = serie
		break
	}

	r.logger.Info("Run complete")
	return nil
}
