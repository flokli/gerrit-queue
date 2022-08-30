package gerrit

import (
	"fmt"

	goGerrit "github.com/andygrunwald/go-gerrit"
	"github.com/apex/log"

	"net/url"
)

// passed to gerrit when retrieving changesets
var additionalFields = []string{
	"LABELS",
	"CURRENT_REVISION",
	"CURRENT_COMMIT",
	"DETAILED_ACCOUNTS",
	"SUBMITTABLE",
}

// IClient defines the gerrit.Client interface
type IClient interface {
	Refresh() error
	GetHEAD() string
	GetBaseURL() string
	GetChangesetURL(changeset *Changeset) string
	SubmitChangeset(changeset *Changeset) (*Changeset, error)
	RebaseChangeset(changeset *Changeset, ref string) (*Changeset, error)
	ChangesetIsRebasedOnHEAD(changeset *Changeset) bool
	ChainIsRebasedOnHEAD(chain *Chain) bool
	FilterChains(filter func(s *Chain) bool) []*Chain
	FindFirstChain(filter func(s *Chain) bool) *Chain
}

var _ IClient = &Client{}

// Client provides some ways to interact with a gerrit instance
type Client struct {
	client      *goGerrit.Client
	logger      *log.Logger
	baseURL     string
	projectName string
	branchName  string
	chains      []*Chain
	head        string
}

// NewClient initializes a new gerrit client
func NewClient(logger *log.Logger, URL, username, password, projectName, branchName string) (*Client, error) {
	urlParsed, err := url.Parse(URL)
	if err != nil {
		return nil, err
	}
	urlParsed.User = url.UserPassword(username, password)

	goGerritClient, err := goGerrit.NewClient(urlParsed.String(), nil)
	if err != nil {
		return nil, err
	}
	return &Client{
		client:      goGerritClient,
		baseURL:     URL,
		logger:      logger,
		projectName: projectName,
		branchName:  branchName,
	}, nil
}

// refreshHEAD queries the commit ID of the selected project and branch
func (c *Client) refreshHEAD() (string, error) {
	branchInfo, _, err := c.client.Projects.GetBranch(c.projectName, c.branchName)
	if err != nil {
		return "", err
	}
	return branchInfo.Revision, nil
}

// GetHEAD returns the internally stored HEAD
func (c *Client) GetHEAD() string {
	return c.head
}

// Refresh causes the client to refresh internal view of gerrit
func (c *Client) Refresh() error {
	c.logger.Debug("refreshing from gerrit")
	HEAD, err := c.refreshHEAD()
	if err != nil {
		return err
	}
	c.head = HEAD

	var queryString = fmt.Sprintf("status:open project:%s branch:%s", c.projectName, c.branchName)
	c.logger.Debugf("fetching changesets: %s", queryString)
	changesets, err := c.fetchChangesets(queryString)
	if err != nil {
		return err
	}

	c.logger.Infof("assembling chains")
	chains, err := AssembleChain(changesets, c.logger)
	if err != nil {
		return err
	}
	chains = SortChains(chains)
	c.chains = chains
	return nil
}

// fetchChangesets fetches a list of changesets matching a passed query string
func (c *Client) fetchChangesets(queryString string) (changesets []*Changeset, Error error) {
	opt := &goGerrit.QueryChangeOptions{}
	opt.Query = []string{
		queryString,
	}
	opt.AdditionalFields = additionalFields
	changes, _, err := c.client.Changes.QueryChanges(opt)
	if err != nil {
		return nil, err
	}

	changesets = make([]*Changeset, 0)
	for _, change := range *changes {
		changesets = append(changesets, MakeChangeset(&change))
	}

	return changesets, nil
}

// fetchChangeset downloads an existing Changeset from gerrit, by its ID
// Gerrit's API is a bit sparse, and only returns what you explicitly ask it
// This is used to refresh an existing changeset with more data.
func (c *Client) fetchChangeset(changeID string) (*Changeset, error) {
	opt := goGerrit.ChangeOptions{}
	opt.AdditionalFields = []string{"LABELS", "DETAILED_ACCOUNTS"}
	changeInfo, _, err := c.client.Changes.GetChange(changeID, &opt)
	if err != nil {
		return nil, err
	}
	return MakeChangeset(changeInfo), nil
}

// SubmitChangeset submits a given changeset, and returns a changeset afterwards.
func (c *Client) SubmitChangeset(changeset *Changeset) (*Changeset, error) {
	changeInfo, _, err := c.client.Changes.SubmitChange(changeset.ChangeID, &goGerrit.SubmitInput{})
	if err != nil {
		return nil, err
	}
	c.head = changeInfo.CurrentRevision
	return c.fetchChangeset(changeInfo.ChangeID)
}

// RebaseChangeset rebases a given changeset on top of a given ref
func (c *Client) RebaseChangeset(changeset *Changeset, ref string) (*Changeset, error) {
	changeInfo, _, err := c.client.Changes.RebaseChange(changeset.ChangeID, &goGerrit.RebaseInput{
		Base: ref,
	})
	if err != nil {
		return changeset, err
	}
	return c.fetchChangeset(changeInfo.ChangeID)
}

// GetBaseURL returns the gerrit base URL
func (c *Client) GetBaseURL() string {
	return c.baseURL
}

// GetProjectName returns the configured gerrit project name
func (c *Client) GetProjectName() string {
	return c.projectName
}

// GetBranchName returns the configured gerrit branch name
func (c *Client) GetBranchName() string {
	return c.branchName
}

// GetChangesetURL returns the URL to view a given changeset
func (c *Client) GetChangesetURL(changeset *Changeset) string {
	return fmt.Sprintf("%s/c/%s/+/%d", c.GetBaseURL(), c.projectName, changeset.Number)
}

// ChangesetIsRebasedOnHEAD returns true if the changeset is rebased on the current HEAD
func (c *Client) ChangesetIsRebasedOnHEAD(changeset *Changeset) bool {
	if len(changeset.ParentCommitIDs) != 1 {
		return false
	}
	return changeset.ParentCommitIDs[0] == c.head
}

// ChainIsRebasedOnHEAD returns true if the whole chain is rebased on the current HEAD
// this is already the case if the first changeset in the chain is rebased on the current HEAD
func (c *Client) ChainIsRebasedOnHEAD(chain *Chain) bool {
	// an empty chain should not exist
	if len(chain.ChangeSets) == 0 {
		return false
	}
	return c.ChangesetIsRebasedOnHEAD(chain.ChangeSets[0])
}

// FilterChains returns a subset of all chains, passing the given filter function
func (c *Client) FilterChains(filter func(s *Chain) bool) []*Chain {
	matchedChains := []*Chain{}
	for _, chain := range c.chains {
		if filter(chain) {
			matchedChains = append(matchedChains, chain)
		}
	}
	return matchedChains
}

// FindFirstChain returns the first chain that matches the filter, or nil if none was found
func (c *Client) FindFirstChain(filter func(s *Chain) bool) *Chain {
	for _, chain := range c.chains {
		if filter(chain) {
			return chain
		}
	}
	return nil
}
