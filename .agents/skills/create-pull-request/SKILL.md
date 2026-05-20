---
name: create-pull-request
description: Create a GitHub pull request for the repository using the repository's standard pull request template.
---

# Create Pull Request Skill

This skill guides the agent to create a GitHub pull request using the repository's official pull request template ([pull_request_template.md](file:///.github/pull_request_template.md)).

## Trigger Conditions
Use this skill when:
- The user requests to create a pull request (PR).
- The user asks to submit/publish code changes to GitHub.
- Code changes have been committed or are ready to be pushed and submitted for review.

## Instructions

1. **Verify Git State:**
   - Run `git status` to ensure all changes are committed.
   - Run `git branch --show-current` to identify the current branch name.
   - Detect the base branch dynamically by running `git remote show origin | grep 'HEAD branch'`, then run `git diff <base-branch>...HEAD` (e.g., `main`, `master`, or whichever the default branch is) to see exactly what changes are being introduced.

2. **Retrieve the Pull Request Template:**
   - Read the template from [.github/pull_request_template.md](file:///.github/pull_request_template.md).

3. **Fill out the Template:**
   - **PR Title:** Create a concise, descriptive title following the repository's commit conventions (e.g., `feat: add login functionality`, `fix: resolve crash on startup`, `docs: update architecture overview`).
   - **Description:** Provide a clear, high-level summary explaining what this PR accomplishes and the rationale behind it.
   - **Type of Change:** Place an `x` only in the checkboxes that apply, leaving the rest unchecked:
     - [ ] Bug fix (non-breaking change which fixes an issue)
     - [ ] New feature (non-breaking change which adds functionality)
     - [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
     - [ ] Documentation update
     - [ ] Refactoring (no functional changes)
   - **Changes Made:** List all modified/added files and a brief summary of the changes in each.
   - **Testing Details:** Explain the verification steps you performed (e.g., ran unit tests, ran the server, manual checks).
   - **Checklist:** Fill out the checklist checkboxes accurately based on what was done (e.g., self-review, test runs).
   - **Related Issues:** Determine the related issue to close automatically:
     - Check the branch name for an issue number using conventional naming patterns (e.g., `fix/11-update-docs`, `issue-11-docs`, or `11-update-docs`). Extract the numeric digits (e.g., `11`).
     - If found, format as `Closes #<issue-number>`.
     - If no issue number can be identified from the branch name, ask the user: *"Which issue (if any) does this PR address?"* and use their response to fill in this section.
   - **Screenshots / Demo:** Include references/links to any generated screenshots or walkthrough recordings if applicable.

4. **Write the PR Body:**
   - Save the filled-in template to a temporary markdown file (e.g., `.pr-body.md`). Do **not** place this file inside the `.git` directory, which is reserved for internal Git metadata.

5. **Execute PR Creation:**
   - Push the local branch to the remote repository (`git push -u origin <branch-name>`).
   - Use the GitHub CLI to create the pull request:
     ```bash
     gh pr create --title "<PR Title>" --body-file .pr-body.md
     ```
   - *Optional:* If requested by the user or if changes are incomplete, add the `--draft` flag.

6. **Cleanup & Verification:**
   - Remove the temporary `.pr-body.md` file.
   - Provide the user with the link to the newly created pull request.
