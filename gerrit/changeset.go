package gerrit

import (
	"bytes"
	"fmt"

	goGerrit "github.com/andygrunwald/go-gerrit"
	"github.com/apex/log"
)

// Changeset represents a single changeset
// Relationships between different changesets are described in Series
type Changeset struct {
	changeInfo      *goGerrit.ChangeInfo
	ChangeID        string
	Number          int
	Verified        int
	CodeReviewed    int
	Autosubmit      int
	Submittable     bool
	CommitID        string
	ParentCommitIDs []string
	OwnerName       string
	Subject         string
}

// MakeChangeset creates a new Changeset object out of a goGerrit.ChangeInfo object
func MakeChangeset(changeInfo *goGerrit.ChangeInfo) *Changeset {
	return &Changeset{
		changeInfo:      changeInfo,
		ChangeID:        changeInfo.ChangeID,
		Number:          changeInfo.Number,
		Verified:        labelInfoToInt(changeInfo.Labels["Verified"]),
		CodeReviewed:    labelInfoToInt(changeInfo.Labels["Code-Review"]),
		Autosubmit:      labelInfoToInt(changeInfo.Labels["Autosubmit"]),
		Submittable:     changeInfo.Submittable,
		CommitID:        changeInfo.CurrentRevision, // yes, this IS the commit ID.
		ParentCommitIDs: getParentCommitIDs(changeInfo),
		OwnerName:       changeInfo.Owner.Name,
		Subject:         changeInfo.Subject,
	}
}

// IsAutosubmit returns true if the changeset is intended to be
// automatically submitted by gerrit-queue.
//
// This is determined by the Change Owner setting +1 on the
// "Autosubmit" label.
func (c *Changeset) IsAutosubmit() bool {
	return c.Autosubmit == 1
}

// IsVerified returns true if the changeset passed CI,
// that's when somebody left the Approved (+1) on the "Verified" label
func (c *Changeset) IsVerified() bool {
	return c.Verified == 1
}

// IsCodeReviewed returns true if the changeset passed code review,
// that's when somebody left the Recommended (+2) on the "Code-Review" label
func (c *Changeset) IsCodeReviewed() bool {
	return c.CodeReviewed == 2
}

func (c *Changeset) String() string {
	var b bytes.Buffer
	b.WriteString("Changeset")
	b.WriteString(fmt.Sprintf("(commitID: %.7s, author: %s, subject: %s, submittable: %v)",
		c.CommitID, c.OwnerName, c.Subject, c.Submittable))
	return b.String()
}

// FilterChangesets filters a list of Changeset by a given filter function
func FilterChangesets(changesets []*Changeset, f func(*Changeset) bool) []*Changeset {
	newChangesets := make([]*Changeset, 0)
	for _, changeset := range changesets {
		if f(changeset) {
			newChangesets = append(newChangesets, changeset)
		} else {
			log.WithField("changeset", changeset.String()).Debug("dropped by filter")
		}
	}
	return newChangesets
}

// labelInfoToInt converts a goGerrit.LabelInfo to -2…+2 int
func labelInfoToInt(labelInfo goGerrit.LabelInfo) int {
	if labelInfo.Recommended.AccountID != 0 {
		return 2
	}
	if labelInfo.Approved.AccountID != 0 {
		return 1
	}
	if labelInfo.Disliked.AccountID != 0 {
		return -1
	}
	if labelInfo.Rejected.AccountID != 0 {
		return -2
	}
	return 0
}

// getParentCommitIDs returns the parent commit IDs of the goGerrit.ChangeInfo
// There is usually only one parent commit ID, except for merge commits.
func getParentCommitIDs(changeInfo *goGerrit.ChangeInfo) []string {
	// obtain the RevisionInfo object
	revisionInfo := changeInfo.Revisions[changeInfo.CurrentRevision]

	// obtain the Commit object
	commit := revisionInfo.Commit

	commitIDs := make([]string, len(commit.Parents))
	for i, commit := range commit.Parents {
		commitIDs[i] = commit.Commit
	}
	return commitIDs
}
