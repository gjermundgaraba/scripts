# Close Reopened Issues

A Node.js script that finds and automatically closes recently reopened issues in a GitHub repository, preserving the previous closure state.

## Features

- Identifies issues that were reopened within a specified time period
- Automatically closes these reopened issues with their previous state reason preserved
- Interactive approval process for each issue before closing
- Option to open issues in browser for review before deciding
- Close comment when closing issues
- Dry run mode to preview what would be closed
- Option to set maximum number of issues to process (useful for testing)

## Requirements

- Node.js
- GitHub Personal Access Token with repo permissions

## Setup

1. Clone this repository
2. Run `npm install` to install dependencies
3. Edit the configuration in `close-reopened-issues.js`
4. Set the `GITHUB_TOKEN` environment variable

## Configuration

Edit the `CONFIG` object in `close-reopened-issues.js`:

```javascript
const CONFIG = {
  owner: 'cosmos',           // GitHub repository owner
  repo: 'ibc-go',            // GitHub repository name
  daysToLookBack: 2,         // Number of days to look back for reopened issues
  closeComment: 'This issue was mistakenly reopened during some integration work, so closing again now.', // Comment when closing
  dryRun: false,             // If true, only print issues without closing them
  maxIssues: 0,              // Maximum number of reopened issues to find and process (0 for unlimited)
};
```

## Usage

1. Set the `GITHUB_TOKEN` environment variable:
   ```
   export GITHUB_TOKEN=your_github_personal_access_token
   ```

2. Run the script:
   ```
   node close-reopened-issues.js
   ```
   or
   ```
   npm start
   ```

## Interactive Mode

By default, the script runs in interactive mode and prompts for confirmation before closing each issue:

- `y` (or Enter) - Close the issue
- `n` - Skip this issue
- `o` - Open the issue in browser for review, then decide
- `a` - Close this and all remaining issues without further prompts

## Example Output

```
$ node close-reopened-issues.js
Looking for issues reopened since 2023-05-07T12:00:00.000Z
Found 42 issues updated since 2023-05-07T12:00:00.000Z
Checking events for issue #123: Example issue title
Issue #123 was reopened within the time period
  Previous state before reopening: completed
Found 3 reopened issues to close.

Issue #123: Example issue title
  URL: https://github.com/cosmos/ibc-go/issues/123
  Reopened at: 2023-05-08T15:23:45Z
  Previous state: completed
Close this issue with state 'completed'? [y=yes, n=skip, o=open in browser first, a=yes to all] y
  Added comment: "This issue was mistakenly reopened during some integration work, so closing again now."
  ✓ Issue #123 closed successfully with state_reason: completed

Completed processing 3 reopened issues.
```

## Customizing

- To run in dry-run mode (no actual closures), set `dryRun: true` in the CONFIG object
- To disable adding a comment, set `closeComment: null` in the CONFIG object
- To change the lookback period, adjust the `daysToLookBack` value
- To limit the number of issues processed (for testing), set `maxIssues` to a positive number