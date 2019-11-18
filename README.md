# gerrit-queue

This daemon automatically rebases and submits changesets from a Gerrit
instance, ensuring they still pass CI.

In a usual gerrit setup with a linear master history, different developers
await CI feedback on a rebased changeset, then one clicks submit, and
effectively makes everybody else rebase again. `gerrit-queue` is meant to
remove these races to master. Developers can add a specific tag to a changeset,
and if all preconditions are met (passing CI and passing Code Review),
`gerrit-queue` takes care of rebasing and submitting it to master.

## How it works
Gerrit only knows about Changesets (and some relations to other changesets),
but usually developers think in terms of multiple changesets.

### The Fetching Phase
`gerrit-queue` fetches all changesets from gerrit, and tries to identify these
chains of changesets. We call them `Series`. All changesets need to have strict
parent/child relationships to be detected.

Series are ordered by the number of changesets in them. This ensures longer
series are merged faster, and less rebases are triggered. In the future, this
might be extended to other metrics.

### The Submit Phase
We loop over all series. If all changesets of a given series pass the required
preconditions (passing CI, passing Code Review, autosubmit Tag) and it's
rebased on top of the current destination branch's HEAD, it can be submitted.

The current serie is removed from the list of series, and we update our HEAD to
the commit ID of the last commit in the submitted series.

### The Rebase Phase
We loop over all remaining series. The first one matching the required
preconditions is rebased on top of the (advanced HEAD) (and if this fails, we
skip and keep trying with the next Series one after another).


These three phases are designed to be stateless, and currently triggered once
per run.
This is supposed to be moved to a more reactive model, so that the submit phase
is triggered by Webhooks for CI feedback, and we don't rebase too often

## Compile and Run
```sh
go generate
GERRIT_PASSWORD=mypassword go run main.go --url https://gerrit.mydomain.com --username myuser --project myproject
```
