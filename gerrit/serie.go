package gerrit

import (
	"fmt"
	"strings"

	"github.com/apex/log"
)

// Serie represents a list of successive changesets with an unbroken parent -> child relation,
// starting from the parent.
type Serie struct {
	ChangeSets []*Changeset
}

// GetParentCommitIDs returns the parent commit IDs
func (s *Serie) GetParentCommitIDs() ([]string, error) {
	if len(s.ChangeSets) == 0 {
		return nil, fmt.Errorf("Can't return parent on a serie with zero ChangeSets")
	}
	return s.ChangeSets[0].ParentCommitIDs, nil
}

// GetLeafCommitID returns the commit id of the last commit in ChangeSets
func (s *Serie) GetLeafCommitID() (string, error) {
	if len(s.ChangeSets) == 0 {
		return "", fmt.Errorf("Can't return leaf on a serie with zero ChangeSets")
	}
	return s.ChangeSets[len(s.ChangeSets)-1].CommitID, nil
}

// CheckIntegrity checks that the series contains a properly ordered and connected chain of commits
func (s *Serie) CheckIntegrity() error {
	logger := log.WithField("serie", s)
	// an empty serie is invalid
	if len(s.ChangeSets) == 0 {
		return fmt.Errorf("An empty serie is invalid")
	}

	previousCommitID := ""
	for i, changeset := range s.ChangeSets {
		// we can't really check the parent of the first commit
		// so skip verifying that one
		logger.WithFields(log.Fields{
			"changeset":        changeset.String(),
			"previousCommitID": fmt.Sprintf("%.7s", previousCommitID),
		}).Debug(" - verifying changeset")

		parentCommitIDs := changeset.ParentCommitIDs
		if len(parentCommitIDs) == 0 {
			return fmt.Errorf("Changesets without any parent are not supported")
		}
		// we don't check parents of the first changeset in a series
		if i != 0 {
			if len(parentCommitIDs) != 1 {
				return fmt.Errorf("Merge commits in the middle of a series are not supported (only at the beginning)")
			}
			if parentCommitIDs[0] != previousCommitID {
				return fmt.Errorf("changesets parent commit id doesn't match previous commit id")
			}
		}
		// update previous commit id for the next loop iteration
		previousCommitID = changeset.CommitID
	}
	return nil
}

// FilterAllChangesets applies a filter function on all of the changesets in the series.
// returns true if it returns true for all changesets, false otherwise
func (s *Serie) FilterAllChangesets(f func(c *Changeset) bool) bool {
	for _, changeset := range s.ChangeSets {
		if f(changeset) == false {
			return false
		}
	}
	return true
}

func (s *Serie) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Serie[%d]", len(s.ChangeSets)))
	if len(s.ChangeSets) == 0 {
		sb.WriteString("()\n")
		return sb.String()
	}
	parentCommitIDs, err := s.GetParentCommitIDs()
	if err == nil {
		if len(parentCommitIDs) == 1 {
			sb.WriteString(fmt.Sprintf("(parent: %.7s)", parentCommitIDs[0]))
		} else {
			sb.WriteString("(merge: ")

			for i, parentCommitID := range parentCommitIDs {
				sb.WriteString(fmt.Sprintf("%.7s", parentCommitID))
				if i < len(parentCommitIDs) {
					sb.WriteString(", ")
				}
			}

			sb.WriteString(")")

		}
	}
	sb.WriteString(fmt.Sprintf("(%.7s..%.7s)",
		s.ChangeSets[0].CommitID,
		s.ChangeSets[len(s.ChangeSets)-1].CommitID))
	return sb.String()
}

func shortCommitID(commitID string) string {
	return commitID[:6]
}
