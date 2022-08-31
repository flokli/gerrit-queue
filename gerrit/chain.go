package gerrit

import (
	"fmt"
	"strings"

	"github.com/apex/log"
)

// Chain represents a list of successive changesets with an unbroken parent -> child relation,
// starting from the parent.
type Chain struct {
	ChangeSets []*Changeset
}

// GetParentCommitIDs returns the parent commit IDs
func (s *Chain) GetParentCommitIDs() ([]string, error) {
	if len(s.ChangeSets) == 0 {
		return nil, fmt.Errorf("can't return parent on a chain with zero ChangeSets")
	}
	return s.ChangeSets[0].ParentCommitIDs, nil
}

// GetLeafCommitID returns the commit id of the last commit in ChangeSets
func (s *Chain) GetLeafCommitID() (string, error) {
	if len(s.ChangeSets) == 0 {
		return "", fmt.Errorf("can't return leaf on a chain with zero ChangeSets")
	}
	return s.ChangeSets[len(s.ChangeSets)-1].CommitID, nil
}

// Validate checks that the chain contains a properly ordered and connected chain of commits
func (s *Chain) Validate() error {
	logger := log.WithField("chain", s)
	// an empty chain is invalid
	if len(s.ChangeSets) == 0 {
		return fmt.Errorf("an empty chain is invalid")
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
			return fmt.Errorf("changesets without any parent are not supported")
		}
		// we don't check parents of the first changeset in a chain
		if i != 0 {
			if len(parentCommitIDs) != 1 {
				return fmt.Errorf("merge commits in the middle of a chain are not supported (only at the beginning)")
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

// AllChangesets applies a filter function on all of the changesets in the chain.
// returns true if it returns true for all changesets, false otherwise
func (s *Chain) AllChangesets(f func(c *Changeset) bool) bool {
	for _, changeset := range s.ChangeSets {
		if !f(changeset) {
			return false
		}
	}
	return true
}

func (s *Chain) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Chain[%d]", len(s.ChangeSets)))
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
