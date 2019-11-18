package gerrit

import (
	"bytes"
	"fmt"

	goGerrit "github.com/andygrunwald/go-gerrit"
	log "github.com/sirupsen/logrus"
)

// Changeset represents a single changeset
// Relationships between different changesets are described in Series
type Changeset struct {
	changeInfo      *goGerrit.ChangeInfo
	ChangeID        string
	Number          int
	IsVerified      bool
	IsCodeReviewed  bool
	HashTags        []string
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
		IsVerified:      isVerified(changeInfo),
		IsCodeReviewed:  isCodeReviewed(changeInfo),
		HashTags:        changeInfo.Hashtags,
		CommitID:        changeInfo.CurrentRevision, // yes, this IS the commit ID.
		ParentCommitIDs: getParentCommitIDs(changeInfo),
		OwnerName:       changeInfo.Owner.Name,
		Subject:         changeInfo.Subject,
	}
}

// MakeMockChangeset creates a mock changeset
// func MakeMockChangeset(isVerified, IsCodeReviewed bool, hashTags []string, commitID string, parentCommitIDs []string, ownerName, subject string) *Changeset {
// 	//TODO impl
// 	return nil
//}

// HasTag returns true if a Changeset has the given tag.
func (c *Changeset) HasTag(tag string) bool {
	hashTags := c.HashTags
	for _, hashTag := range hashTags {
		if hashTag == tag {
			return true
		}
	}
	return false
}

func (c *Changeset) String() string {
	var b bytes.Buffer
	b.WriteString("Changeset")
	b.WriteString(fmt.Sprintf("(commitID: %.7s, author: %s, subject: %s)", c.CommitID, c.OwnerName, c.Subject))
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

// isVerified returns true if the code passed CI,
// that's when somebody left the Approved (+1) on the "Verified" label
func isVerified(changeInfo *goGerrit.ChangeInfo) bool {
	labels := changeInfo.Labels
	return labels["Verified"].Approved.AccountID != 0
}

// isCodeReviewed returns true if the code passed code review,
// that's when somebody left the Recommended (+2) on the "Code-Review" label
func isCodeReviewed(changeInfo *goGerrit.ChangeInfo) bool {
	labels := changeInfo.Labels
	return labels["Code-Review"].Recommended.AccountID != 0
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
