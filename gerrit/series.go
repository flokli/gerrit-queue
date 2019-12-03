package gerrit

import (
	"sort"

	"github.com/apex/log"
)

// AssembleSeries consumes a list of `Changeset`, and groups them together to series
//
// We initially put every Changeset in its own Serie
//
// As we have no control over the order of the passed changesets,
// we maintain a lookup table, mapLeafToSerie,
// which allows to lookup a serie by its leaf commit id
// We concat series in a fixpoint approach
// because both appending and prepending is much more complex.
// Concatenation moves changesets of the later changeset in the previous one
// in a cleanup phase, we remove orphaned series (those without any changesets inside)
// afterwards, we do an integrity check, just to be on the safe side.
func AssembleSeries(changesets []*Changeset, logger *log.Logger) ([]*Serie, error) {
	series := make([]*Serie, 0)
	mapLeafToSerie := make(map[string]*Serie, 0)

	for _, changeset := range changesets {
		l := logger.WithField("changeset", changeset.String())

		l.Debug("creating initial serie")
		serie := &Serie{
			ChangeSets: []*Changeset{changeset},
		}
		series = append(series, serie)
		mapLeafToSerie[changeset.CommitID] = serie
	}

	// Combine series using a fixpoint approach, with a max iteration count.
	logger.Debug("glueing together phase")
	for i := 1; i < 100; i++ {
		didUpdate := false
		logger.Debugf("at iteration %d", i)
		for j, serie := range series {
			l := logger.WithFields(log.Fields{
				"i":     i,
				"j":     j,
				"serie": serie.String(),
			})
			parentCommitIDs, err := serie.GetParentCommitIDs()
			if err != nil {
				return series, err
			}
			if len(parentCommitIDs) != 1 {
				// We can't append merge commits to other series
				l.Infof("No single parent, skipping.")
				continue
			}
			parentCommitID := parentCommitIDs[0]
			l.Debug("Looking for a predecessor.")
			// if there's another serie that has this parent as a leaf, glue together
			if otherSerie, ok := mapLeafToSerie[parentCommitID]; ok {
				if otherSerie == serie {
					continue
				}
				l = l.WithField("otherSerie", otherSerie)

				myLeafCommitID, err := serie.GetLeafCommitID()
				if err != nil {
					return series, err
				}

				// append our changesets to the other serie
				l.Debug("Splicing together.")
				otherSerie.ChangeSets = append(otherSerie.ChangeSets, serie.ChangeSets...)

				delete(mapLeafToSerie, parentCommitID)
				mapLeafToSerie[myLeafCommitID] = otherSerie

				// orphan our serie
				serie.ChangeSets = []*Changeset{}
				// remove the orphaned serie from the lookup table
				delete(mapLeafToSerie, myLeafCommitID)

				didUpdate = true
			} else {
				l.Debug("Not found.")
			}
		}
		series = removeOrphanedSeries(series)
		if !didUpdate {
			logger.Infof("converged after %d iterations", i)
			break
		}
	}

	// Check integrity, just to be on the safe side.
	for _, serie := range series {
		l := logger.WithField("serie", serie.String())
		l.Debugf("checking integrity")
		err := serie.CheckIntegrity()
		if err != nil {
			l.Errorf("checking integrity failed: %s", err)
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
