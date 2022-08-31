package submitqueue

import (
	"fmt"
	"sync"

	"github.com/apex/log"

	"github.com/flokli/gerrit-queue/gerrit"
)

// Runner is a struct existing across the lifetime of a single run of the submit queue
// it contains a mutex to avoid being run multiple times.
// In fact, it even cancels runs while another one is still in progress.
// It contains a Gerrit object facilitating access, a log object, the configured submit queue tag
// and a `wipChain` (only populated if waiting for a rebase)
type Runner struct {
	mut              sync.Mutex
	currentlyRunning bool
	wipChain         *gerrit.Chain
	logger           *log.Logger
	gerrit           *gerrit.Client
}

// NewRunner creates a new Runner struct
func NewRunner(logger *log.Logger, gerrit *gerrit.Client) *Runner {
	return &Runner{
		logger: logger,
		gerrit: gerrit,
	}
}

// isAutoSubmittable determines if something could be autosubmitted, potentially requiring a rebase
// for this, it needs to:
//   - have the "Autosubmit" label set to +1
//   - have gerrit's 'submittable' field set to true
//
// it doesn't check if the chain is rebased on HEAD
func (r *Runner) isAutoSubmittable(s *gerrit.Chain) bool {
	for _, c := range s.ChangeSets {
		if !c.Submittable || !c.IsAutosubmit() {
			return false
		}
	}
	return true
}

// IsCurrentlyRunning returns true if the runner is currently running
func (r *Runner) IsCurrentlyRunning() bool {
	return r.currentlyRunning
}

// GetWIPChain returns the current wipChain, if any, nil otherwiese
// Acquires a lock, so check with IsCurrentlyRunning first
func (r *Runner) GetWIPChain() *gerrit.Chain {
	r.mut.Lock()
	defer func() {
		r.mut.Unlock()
	}()
	return r.wipChain
}

// Trigger gets triggered periodically
func (r *Runner) Trigger(fetchOnly bool) error {
	// TODO: If CI fails, remove the auto-submit labels => rules.pl
	// Only one trigger can run at the same time
	r.mut.Lock()
	if r.currentlyRunning {
		return fmt.Errorf("already running, skipping")
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

	if r.wipChain != nil {
		// refresh wipChain with how it looks like in gerrit now
		wipChain := r.gerrit.FindFirstChain(func(s *gerrit.Chain) bool {
			// the new wipChain needs to have the same number of changesets
			if len(r.wipChain.ChangeSets) != len(s.ChangeSets) {
				return false
			}
			// … and the same ChangeIDs.
			for idx, c := range s.ChangeSets {
				if r.wipChain.ChangeSets[idx].ChangeID != c.ChangeID {
					return false
				}
			}
			return true
		})
		if wipChain == nil {
			r.logger.WithField("wipChain", r.wipChain).Warn("wipChain has disappeared")
			r.wipChain = nil
		} else {
			r.wipChain = wipChain
		}
	}

	for {
		// initialize logger
		r.logger.Info("Running")
		if r.wipChain != nil {
			// if we have a wipChain
			l := r.logger.WithField("wipChain", r.wipChain)
			l.Info("Checking wipChain")

			// discard wipChain not rebased on HEAD
			// we rebase them at the end of the loop, so this means master advanced without going through the submit queue
			if !r.gerrit.ChainIsRebasedOnHEAD(r.wipChain) {
				l.Warnf("HEAD has moved to %v while still waiting for wipChain, discarding it", r.gerrit.GetHEAD())
				r.wipChain = nil
				continue
			}

			// we now need to check CI feedback:
			// wipChain might have failed CI in the meantime
			for _, c := range r.wipChain.ChangeSets {
				if c == nil {
					l.Error("BUG: changeset is nil")
					continue
				}
				if c.Verified < 0 {
					l.WithField("failingChangeset", c).Warnf("wipChain failed CI in the meantime, discarding.")
					r.wipChain = nil
					continue
				}
			}

			// it might still be waiting for CI
			for _, c := range r.wipChain.ChangeSets {
				if c == nil {
					l.Error("BUG: changeset is nil")
					continue
				}
				if c.Verified == 0 {
					l.WithField("pendingChangeset", c).Warnf("still waiting for CI feedback in wipChain, going back to sleep.")
					// break the loop, take a look at it at the next trigger.
					return nil
				}
			}

			// it might be autosubmittable
			if r.isAutoSubmittable(r.wipChain) {
				l.Infof("submitting wipChain")
				// if the WIP changeset is ready (auto submittable and rebased on HEAD), submit
				for _, changeset := range r.wipChain.ChangeSets {
					_, err := r.gerrit.SubmitChangeset(changeset)
					if err != nil {
						l.WithField("changeset", changeset).Error("error submitting changeset")
						r.wipChain = nil
						return err
					}
				}
				r.wipChain = nil
			} else {
				l.Error("BUG: wipChain is not autosubmittable")
				r.wipChain = nil
			}
		}

		r.logger.Info("Looking for chains ready to submit")
		// Find chain, that:
		//  * has the auto-submit label
		//  * has +2 review
		//  * has +1 CI
		//  * is rebased on master
		chain := r.gerrit.FindFirstChain(func(s *gerrit.Chain) bool {
			return r.isAutoSubmittable(s) && s.ChangeSets[0].ParentCommitIDs[0] == r.gerrit.GetHEAD()
		})
		if chain != nil {
			r.logger.WithField("chain", chain).Info("Found chain to submit without necessary rebase")
			r.wipChain = chain
			continue
		}

		// Find chain, that:
		//  * has the auto-submit label
		//  * has +2 review
		//  * has +1 CI
		//  * is NOT rebased on master
		chain = r.gerrit.FindFirstChain(r.isAutoSubmittable)
		if chain == nil {
			r.logger.Info("no more submittable chain found, going back to sleep.")
			break
		}

		l := r.logger.WithField("chain", chain)
		l.Info("found chain, which needs a rebase")
		// TODO: move into Client.RebaseChangeset function
		head := r.gerrit.GetHEAD()
		for _, changeset := range chain.ChangeSets {
			changeset, err := r.gerrit.RebaseChangeset(changeset, head)
			if err != nil {
				l.Error(err.Error())
				return err
			}
			head = changeset.CommitID
		}
		// we don't need to care about updating the rebased changesets or getting the updated HEAD,
		// as we'll refetch it on the beginning of the next trigger anyways
		r.wipChain = chain
		break
	}

	r.logger.Info("Run complete")
	return nil
}
