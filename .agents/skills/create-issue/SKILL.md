---
name: create-issue
description: Create a GitHub issue for the repository by selecting the appropriate issue template (bug report, documentation, feature request, or improvement request), filling in the relevant fields, and submitting via the GitHub CLI.
---

# Create Issue Skill

This skill guides the agent to create a GitHub issue using one of the repository's official issue templates located in [.github/ISSUE_TEMPLATE/](file:///.github/ISSUE_TEMPLATE/).

## Available Templates

| # | Template                 | File                      | Label           |
|---|--------------------------|---------------------------|-----------------|
| 1 | 🐞 Bug Report            | `bug_report.yml`          | `bug`           |
| 2 | 📝 Documentation Request | `documentation.yml`       | `documentation` |
| 3 | ✨ Feature Request        | `feature_request.yml`     | `enhancement`   |
| 4 | 🚀 Improvement Request   | `improvement_request.yml` | `enhancement`   |

## Trigger Conditions
Use this skill when:
- The user asks to create, file, open, or raise an issue.
- The user wants to report a bug, request a feature, suggest an improvement, or request a documentation update.

## Instructions

### Step 1: Resolve Valid Labels
Before creating the issue, always run `gh label list` to get the exact labels available in this repository. Map the template's intended label to the closest matching one from the actual list. Never guess or use a label name that has not been confirmed from this output.

### Step 2: Select the Template
If the user has not already indicated which template to use, infer it from the context:
- Bug or error → 🐞 Bug Report
- Missing or incorrect docs → 📝 Documentation Request
- New capability or feature → ✨ Feature Request
- Existing behaviour that could be better → 🚀 Improvement Request

If the context is ambiguous, present the four options and ask the user to choose.

### Step 3: Fill in the Template Fields

#### 🐞 Bug Report
- **Describe the Bug** *(required)* — Clear description of the bug.
- **To Reproduce** *(required)* — Numbered steps to reproduce the issue.
- **Expected Behavior** *(required)* — What should have happened.
- **Screenshots** — Drag-and-drop images or links if applicable.
- **Version** *(required)* — Product version or commit hash.
- **Environment Details** — OS, database, Docker version, etc.
- **Additional Context** — Any other relevant information.

#### 📝 Documentation Request
- **Affected Documentation / Link** *(required)* — Paths or links to the docs that need updating (use repository-relative paths, e.g. `README.md`, `backend/docs/architecture.md`).
- **Proposed Changes** *(required)* — What should be updated, corrected, or added.
- **Context or Reason** — Why this update is needed.

#### ✨ Feature Request
- **Problem** *(required)* — What problem or gap this feature will solve.
- **Proposed Solution** *(required)* — Description of the desired feature.
- **Alternatives** — Other approaches or workarounds considered.

#### 🚀 Improvement Request
- **Current Limitation** *(required)* — What is limited or sub-optimal today.
- **Suggested Improvement** *(required)* — What could be improved and how.
- **Version** — Product version or commit hash.
- **Additional Context** — Anything else relevant.

### Step 4: Write the Issue Body File
Save the filled-in body (without the YAML frontmatter — plain markdown only) to a temporary file, e.g. `issue-body.md` at the workspace root.

### Step 5: Create the Issue
Run:
```bash
gh issue create \
  --title "<Concise, action-oriented title>" \
  --label "<confirmed-label-from-step-1>" \
  --body-file issue-body.md
```

### Step 6: Cleanup
- Delete the temporary `issue-body.md` file after the issue is created.
- Share the issue URL with the user.
