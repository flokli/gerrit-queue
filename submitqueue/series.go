package submitqueue

import (
	"sort"

	"github.com/tweag/gerrit-queue/gerrit"

	"github.com/sirupsen/logrus"
)

// AssembleSeries consumes a list of `Changeset`, and groups them together to series
//
// As we have no control over the order of the passed changesets,
// we maintain two lookup tables,
// mapLeafToSerie, which allows to lookup a serie by its leaf commit id,
// to append to an existing serie
// and mapParentToSeries, which allows to lookup all series having a certain parent commit id,
// to prepend to any of the existing series
// if we can't find anything, we create a new series
func AssembleSeries(changesets []*gerrit.Changeset, log *logrus.Logger) ([]*Serie, error) {
	series := make([]*Serie, 0)
	mapLeafToSerie := make(map[string]*Serie, 0)

	for _, changeset := range changesets {
		logger := log.WithFields(logrus.Fields{
			"changeset": changeset.String(),
		})

		logger.Debug("creating initial serie")
		serie := &Serie{
			ChangeSets: []*gerrit.Changeset{changeset},
		}
		series = append(series, serie)
		mapLeafToSerie[changeset.CommitID] = serie
	}

	// Combine series using a fixpoint approach, with a max iteration count.
	log.Debug("glueing together phase")
	for i := 1; i < 100; i++ {
		didUpdate := false
		log.Debugf("at iteration %d", i)
		for _, serie := range series {
			logger := log.WithField("serie", serie.String())
			parentCommitIDs, err := serie.GetParentCommitIDs()
			if err != nil {
				return series, err
			}
			if len(parentCommitIDs) != 1 {
				// We can't append merge commits to other series
				logger.Infof("No single parent, skipping.")
				continue
			}
			parentCommitID := parentCommitIDs[0]
			logger.Debug("Looking for a predecessor.")
			// if there's another serie that has this parent as a leaf, glue together
			if otherSerie, ok := mapLeafToSerie[parentCommitID]; ok {
				if otherSerie == serie {
					continue
				}
				logger := logger.WithField("otherSerie", otherSerie)

				myLeafCommitID, err := serie.GetLeafCommitID()
				if err != nil {
					return series, err
				}

				// append our changesets to the other serie
				logger.Debug("Splicing together.")
				otherSerie.ChangeSets = append(otherSerie.ChangeSets, serie.ChangeSets...)

				delete(mapLeafToSerie, parentCommitID)
				mapLeafToSerie[myLeafCommitID] = otherSerie

				// orphan our serie
				serie.ChangeSets = []*gerrit.Changeset{}
				// remove the orphaned serie from the lookup table
				delete(mapLeafToSerie, myLeafCommitID)

				didUpdate = true
			} else {
				logger.Debug("Not found.")
			}
		}
		series = removeOrphanedSeries(series)
		if !didUpdate {
			log.Infof("converged after %d iterations", i)
			break
		}
	}

	// Check integrity, just to be on the safe side.
	for _, serie := range series {
		logger := log.WithFields(logrus.Fields{
			"serie": serie.String(),
		})
		logger.Debugf("checking integrity")
		err := serie.CheckIntegrity()
		if err != nil {
			logger.Errorf("checking integrity failed: %s", err)
		}
	}
	return series, nil
}

// removeOrphanedSeries removes all empty series (that contain zero changesets)
func removeOrphanedSeries(series []*Serie) []*Serie {
	newSeries := []*Serie{}
	for _, serie := range series {
		if len(serie.ChangeSets) != 0 {
			newSeries = append(newSeries, serie)
		}
	}
	return newSeries
}

// SortSeries sorts a list of series by the number of changesets in each serie, descending
func SortSeries(series []*Serie) []*Serie {
	newSeries := make([]*Serie, len(series))
	copy(newSeries, series)
	sort.Slice(newSeries, func(i, j int) bool {
		// the weight depends on the amount of changesets series changeset size
		return len(series[i].ChangeSets) > len(series[j].ChangeSets)
	})
	return newSeries
}
