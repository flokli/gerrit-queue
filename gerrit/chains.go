package gerrit

import (
	"sort"

	"github.com/apex/log"
)

// AssembleChain consumes a list of changesets, and groups them together to chains.
//
// Initially, every changeset is put in its own individual chain.
//
// we maintain a lookup table, mapLeafToChain,
// which allows to lookup a chain by its leaf commit id
// We concat chains in a fixpoint approach
// because both appending and prepending is much more complex.
// Concatenation moves changesets of the later changeset in the previous one
// in a cleanup phase, we remove orphaned chains (those without any changesets inside)
// afterwards, we do an integrity check, just to be on the safe side.
func AssembleChain(changesets []*Changeset, logger *log.Logger) ([]*Chain, error) {
	chains := make([]*Chain, 0)
	mapLeafToChain := make(map[string]*Chain, 0)

	for _, changeset := range changesets {
		l := logger.WithField("changeset", changeset.String())

		l.Debug("creating initial chain")
		chain := &Chain{
			ChangeSets: []*Changeset{changeset},
		}
		chains = append(chains, chain)
		mapLeafToChain[changeset.CommitID] = chain
	}

	// Combine chain using a fixpoint approach, with a max iteration count.
	logger.Debug("glueing together phase")
	for i := 1; i < 100; i++ {
		didUpdate := false
		logger.Debugf("at iteration %d", i)
		for j, chain := range chains {
			l := logger.WithFields(log.Fields{
				"i":     i,
				"j":     j,
				"chain": chain.String(),
			})
			parentCommitIDs, err := chain.GetParentCommitIDs()
			if err != nil {
				return chains, err
			}
			if len(parentCommitIDs) != 1 {
				// We can't append merge commits to other chains
				l.Infof("No single parent, skipping.")
				continue
			}
			parentCommitID := parentCommitIDs[0]
			l.Debug("Looking for a predecessor.")
			// if there's another chain that has this parent as a leaf, glue together
			if otherChain, ok := mapLeafToChain[parentCommitID]; ok {
				if otherChain == chain {
					continue
				}
				l = l.WithField("otherChain", otherChain)

				myLeafCommitID, err := chain.GetLeafCommitID()
				if err != nil {
					return chains, err
				}

				// append our changesets to the other chain
				l.Debug("Splicing together.")
				otherChain.ChangeSets = append(otherChain.ChangeSets, chain.ChangeSets...)

				delete(mapLeafToChain, parentCommitID)
				mapLeafToChain[myLeafCommitID] = otherChain

				// orphan our chain
				chain.ChangeSets = []*Changeset{}
				// remove the orphaned chain from the lookup table
				delete(mapLeafToChain, myLeafCommitID)

				didUpdate = true
			} else {
				l.Debug("Not found.")
			}
		}
		chains = removeOrphanedChains(chains)
		if !didUpdate {
			logger.Infof("converged after %d iterations", i)
			break
		}
	}

	// Check integrity, just to be on the safe side.
	for _, chain := range chains {
		l := logger.WithField("chain", chain.String())
		l.Debugf("checking integrity")
		err := chain.Validate()
		if err != nil {
			l.Errorf("checking integrity failed: %s", err)
		}
	}
	return chains, nil
}

// removeOrphanedChains removes all empty chains (that contain zero changesets)
func removeOrphanedChains(chains []*Chain) []*Chain {
	newChains := []*Chain{}
	for _, chain := range chains {
		if len(chain.ChangeSets) != 0 {
			newChains = append(newChains, chain)
		}
	}
	return newChains
}

// SortChains sorts a list of chains by the number of changesets in each chain, descending
func SortChains(chains []*Chain) []*Chain {
	newChains := make([]*Chain, len(chains))
	copy(newChains, chains)
	sort.Slice(newChains, func(i, j int) bool {
		// the weight depends on the amount of changesets in the chain
		return len(chains[i].ChangeSets) > len(chains[j].ChangeSets)
	})
	return newChains
}
