# gerrit-queue

This daemon automatically rebases and submits changesets from a Gerrit
instance, ensuring they still pass CI.

In a usual gerrit setup with a linear master history, different developers
await CI feedback on a rebased changeset, then one clicks submit, and
effectively makes everybody else rebase again. `gerrit-queue` is meant to
remove these races to master.

Developers can set the `Autosubmit` customized label to `+1` on all changesets
in a chain, and if all preconditions on are met ("submittable" in gerrit
speech, this usually means passing CI and passing Code Review),
`gerrit-queue` takes care of rebasing and submitting it to master.

Refer to the [Customized Label Gerrit docs](https://gerrit-review.googlesource.com/Documentation/config-labels.html#label_custom)
on how to create the `Autosubmit` label and configure permissions, but
something like the following in your projects `project.config` should suffice:

```
[access "refs/*"]
  # […]
	#
	# Set exclusive because only the change owner should be able to do this:
	exclusiveGroupPermissions = label-Autosubmit
	label-Autosubmit = +0..+1 group Change Owner
[label "Autosubmit"]
	function = NoOp
	value = 0 Submit manually
	value = +1 Submit automatically
	allowPostSubmit = false
	copyAnyScore = true
	defaultValue = 0
```

Note that if a setup uses `rules.pl`, the label will not be rendered unless it
is configured as `may(_)` in the rules.

See [TVL CL 4241](https://cl.tvl.fyi/c/depot/+/4241) for an example.

## How it works
Gerrit only knows about Changesets (and some relations to other changesets),
but usually developers think in terms of multiple changesets.

### Fetching changesets
`gerrit-queue` fetches all changesets from gerrit, and tries to identify these
chains of changesets.
All changesets need to have strict parent/child relationships to be detected.
(This means, if a user manually rebases half of a chain through the Gerrit Web
Interface, these will be considered as two independent chains!)

Chains are rebased by the number of changesets in them. This ensures longer
chains are merged faster, and less rebases are triggered. In the future, this
might be extended to other strategies.

### Submitting changesets
The submitqueue has a Trigger() function, which gets periodically executed.

It can keep a reference to one single chain across multiple runs. This is
necessary if it previously rebased one chain to current HEAD and needs to wait
some time until CI feedback is there. If it wouldn't keep that state, it would
pick another chain (with +1 from CI) and trigger a rebase on that one, so
depending on CI run times and trigger intervals, if not keepig this information
it'd end up rebasing all unrebased changesets on the same HEAD, and then just
pick one, instead of waiting for the one to finish.

The Trigger() function first instructs the gerrit client to fetch changesets
and assemble chains.
If there is a `wipChain` from a previous run, we check if it can still be found
in the newly assembled list of chains (it still needs to contain the same
number of changesets. Commit IDs may differ, because the code doesn't reassemble
a `wipChain` after scheduling a rebase.
If the `wipChain` could be refreshed, we update the pointer with the newly
assembled chain. If we couldn't find it, we drop it.

Now, we enter the main for loop. The first half of the loop checks various
conditions of the current `wipChain`, and if successful, does the submit
("Submit phase"), the second half will pick a suitable new `wipChain`, and
potentially do a rebase ("Pick phase").

#### Submit phase
We check if there is an existing `wipChain`. If there isn't, we immediately go to
the "pick" phase.

The `wipChain` still needs to be rebased on `HEAD` (otherwise, the submit queue
advanced outside of gerrit), and should not fail CI (logical merge conflict) -
otherwise we discard it, and continue with the picking phase.

If the `wipChain` still contains a changeset awaiting CI feedback, we `return`
from the `Trigger()` function (and go back to sleep).

If the changeset is "submittable" in gerrit speech, and has the necessary
submit queue tag set, we submit it.

#### Pick phase
The pick phase finds a new `wipChain`. It'll first try to find one that already
is rebased on the current `HEAD` (so the loop can just continue, and the next
submit phase simply submit), and otherwise fall back to a not-yet-rebased
chain. Because the rebase mandates waiting for CI, the code `return`s the
`Trigger()` function, so it'll be called again after waiting some time.

## Compile and Run
```sh
go generate
GERRIT_PASSWORD=mypassword go run main.go --url https://gerrit.mydomain.com --username myuser --project myproject
```
