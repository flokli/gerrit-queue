package gerrit

import (
	goGerrit "github.com/andygrunwald/go-gerrit"

	"net/url"
)

// passed to gerrit when retrieving changesets
var additionalFields = []string{"LABELS", "CURRENT_REVISION", "CURRENT_COMMIT", "DETAILED_ACCOUNTS"}

// IClient defines the gerrit.Client interface
type IClient interface {
	SearchChangesets(queryString string) (changesets []*Changeset, Error error)
	GetHEAD(projectName string, branchName string) (string, error)
	GetChangeset(changeID string) (*Changeset, error)
	SubmitChangeset(changeset *Changeset) (*Changeset, error)
	RebaseChangeset(changeset *Changeset, ref string) (*Changeset, error)
	RemoveTag(changeset *Changeset, tag string) (*Changeset, error)
}

var _ IClient = &Client{}

// Client provides some ways to interact with a gerrit instance
type Client struct {
	client *goGerrit.Client
}

// NewClient initializes a new gerrit client
func NewClient(URL, username, password string) (*Client, error) {
	urlParsed, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	urlParsed.User = url.UserPassword(username, password)

	goGerritClient, err := goGerrit.NewClient(urlParsed.String(), nil)
	if err != nil {
		return nil, err
	}
	return &Client{client: goGerritClient}, nil
}

// SearchChangesets fetches a list of changesets matching a passed query string
func (gerrit *Client) SearchChangesets(queryString string) (changesets []*Changeset, Error error) {
	opt := &goGerrit.QueryChangeOptions{}
	opt.Query = []string{
		queryString,
	}
	opt.AdditionalFields = additionalFields //TODO: check DETAILED_ACCOUNTS is needed
	changes, _, err := gerrit.client.Changes.QueryChanges(opt)
	if err != nil {
		return nil, err
	}

	changesets = make([]*Changeset, 0)
	for _, change := range *changes {
		changesets = append(changesets, MakeChangeset(&change))
	}

	return changesets, nil
}

// GetHEAD returns the commit ID of a selected branch
func (gerrit *Client) GetHEAD(projectName string, branchName string) (string, error) {
	branchInfo, _, err := gerrit.client.Projects.GetBranch(projectName, branchName)
	if err != nil {
		return "", err
	}
	return branchInfo.Revision, nil
}

// GetChangeset downloads an existing Changeset from gerrit, by its ID
// Gerrit's API is a bit sparse, and only returns what you explicitly ask it
// This is used to refresh an existing changeset with more data.
func (gerrit *Client) GetChangeset(changeID string) (*Changeset, error) {
	opt := goGerrit.ChangeOptions{}
	opt.AdditionalFields = []string{"LABELS", "DETAILED_ACCOUNTS"}
	changeInfo, _, err := gerrit.client.Changes.GetChange(changeID, &opt)
	if err != nil {
		return nil, err
	}
	return MakeChangeset(changeInfo), nil
}

// SubmitChangeset submits a given changeset, and returns a changeset afterwards.
func (gerrit *Client) SubmitChangeset(changeset *Changeset) (*Changeset, error) {
	changeInfo, _, err := gerrit.client.Changes.SubmitChange(changeset.ChangeID, &goGerrit.SubmitInput{})
	if err != nil {
		return nil, err
	}
	return gerrit.GetChangeset(changeInfo.ChangeID)
}

// RebaseChangeset rebases a given changeset on top of a given ref
func (gerrit *Client) RebaseChangeset(changeset *Changeset, ref string) (*Changeset, error) {
	changeInfo, _, err := gerrit.client.Changes.RebaseChange(changeset.ChangeID, &goGerrit.RebaseInput{
		Base: ref,
	})
	if err != nil {
		return changeset, err
	}
	return gerrit.GetChangeset(changeInfo.ChangeID)
}

// RemoveTag removes the submit queue tag from a changeset and updates gerrit
// we never add, that's something users should do in the GUI.
func (gerrit *Client) RemoveTag(changeset *Changeset, tag string) (*Changeset, error) {
	hashTags := changeset.HashTags
	newHashTags := []string{}
	for _, hashTag := range hashTags {
		if hashTag != tag {
			newHashTags = append(newHashTags, hashTag)
		}
	}
	// TODO: implement set hashtags api in go-gerrit and use here
	// https://gerrit-review.googlesource.com/Documentation/rest-api-changes.html#set-hashtags
	return changeset, nil
}
