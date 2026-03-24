# Rancher Turtles Release Notes

Commit messages are usually hard to understand for those that are not very familiar with the project and we must add a "human-readable" description of what the release brings at the beginning of release notes. [This](https://github.com/rancher/turtles/releases/tag/v0.26.0) can be used for reference.

If the new release includes breaking changes or requires any sort of manual steps from the user, make sure these are clearly stated at the top of the release notes.

We use [releasepost](https://github.com/updatecli/releasepost) which is part of `updatecli` to [fetch notes](https://github.com/rancher/turtles-product-docs/blob/main/updatecli-compose.yaml) from `rancher/turtles` releases and embed them into the documentation.

This approach is inspired by [Fleet Docs](https://github.com/rancher/fleet-docs/blob/5b7da0f8b5d19a28d90eeaec98a94c7b763eaea3/updatecli-compose.yaml#L16) and it allows us to replicate what's put together in the GitHub release description.

To make release notes more readable and understandable, we adhere to the following recommendations:

- Avoid including the compilation of all changes (commits) that auto-generated release notes provide. This is available in the link to the full changelog.
  - Automatically-generated release notes should only be used as a reference for collecting all recent changes.
- Create more user-understandable groups of changes. See [reference template](#reference-template).
- Remove description of internal changes that are not relevant to users.

## Reference Template

```markdown
# Rancher Turtles - Cluster API integration in Rancher

<brief-summary-of-the-release>

## Notable changes

## Additions

## Bugfixes

## New Contributors

**Full Changelog**: <automatically-generated>
```
