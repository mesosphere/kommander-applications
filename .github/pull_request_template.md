**What problem does this PR solve?**:


**Which issue(s) does this PR fix?**:
<!-- Add a link to the JIRA issue below-->


**Special notes for your reviewer**:
<!--
Use this to provide any additional information to the reviewers.
This may include:
- Manual testing steps.
- Best way to review the PR.
- Where the author wants the most review attention on.
- etc.
-->


**Does this PR introduce a user-facing change?**:
<!--
If yes, add a message in the 'release-note' block below.
If this PR fixes a COPS ticket, include it after the note like: "CLI: Some bug fix. (COPS-xxxx)"
-->
```release-note

```

**Checklist**
<!--
For example, If a chart changes license from say Apache License to GNU AFFERO GENERAL PUBLIC LICENSE then
that would have legal repercussions (as we ship helm charts, image bundles for airgapped etc.,) and multiple
parties (Like Product, Legal for example) need to be notified when such a change happens.
-->

- [ ] If the PR adds a version bump, ensure there is no breaking change in Licensing model (or NA).
- [ ] If a chart is changed or app configuration is significantly changed, the chart version is correctly incremented (so that apps are not automatically upgraded from a previous version of DKP).
