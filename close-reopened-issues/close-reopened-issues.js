#!/usr/bin/env node

const { Octokit } = require('@octokit/rest');
const readline = require('readline');
const open = require('open');

const CONFIG = {
  owner: 'cosmos',           // GitHub repository owner
  repo: 'ibc-go',             // GitHub repository name
  daysToLookBack: 2,        // Number of days to look back for reopened issues
  closeComment: 'This issue was mistakenly reopened during some integration work, so closing again now.', // Comment when closing
  dryRun: false,            // If true, only print issues without closing them
  maxIssues: 0, // Maximum number of reopened issues to find and process (useful for testing). Set to 0 to disable.
};

// Initialize GitHub client
function initializeGitHubClient() {
  const token = process.env.GITHUB_TOKEN;
  
  if (!token) {
    console.error('Error: GitHub token is required. Set the GITHUB_TOKEN environment variable.');
    process.exit(1);
  }
  
  return new Octokit({ auth: token });
}

// Find reopened issues within the time period
async function findReopenedIssues(octokit) {
  // Calculate the date X days ago
  const lookbackDate = new Date();
  lookbackDate.setDate(lookbackDate.getDate() - CONFIG.daysToLookBack);
  const sinceDate = lookbackDate.toISOString();

  console.log(`Looking for issues reopened since ${sinceDate}`);

  // Fetch all issues updated since our lookback date
  const issues = await octokit.paginate(octokit.rest.issues.listForRepo, {
    owner: CONFIG.owner,
    repo: CONFIG.repo,
    state: 'open',
    since: sinceDate,
    per_page: 100
  });


  console.log(`Found ${issues.length} issues updated since ${sinceDate}`);

  // To find reopened issues, we need to check the timeline events
  const reopenedIssues = [];

  for (const issue of issues) {
    console.log(`Checking events for issue #${issue.number}: ${issue.title}`);

    // TODO: REMOVE AFTER TESTING
    if (CONFIG.maxIssues != 0 && reopenedIssues.length == CONFIG.maxIssues) {
      console.log(`  Skipping further checks for testing purposes after finding #${reopenedIssues.length} reopened issues.`);
      break;
    }

    try {
      // Get timeline events for the issue
      const events = await octokit.paginate(octokit.rest.issues.listEventsForTimeline, {
        owner: CONFIG.owner,
        repo: CONFIG.repo,
        issue_number: issue.number,
        per_page: 100
      });

      // Find reopened events within our time window
      const reopenEvents = events.filter(event => {
        return event.event === 'reopened' &&
               new Date(event.created_at) >= lookbackDate;
      });

      if (reopenEvents.length > 0) {
        console.log(`Issue #${issue.number} was reopened within the time period`);

        // Find the previous state before the issue was reopened
        // We need to get the closed event that happened before the reopen
        const lastReopenEvent = reopenEvents[reopenEvents.length - 1];
        const lastReopenIndex = events.findIndex(e =>
          e.id === lastReopenEvent.id
        );

        // Get all events before the reopen
        const previousEvents = events.slice(0, lastReopenIndex);

        // Find the most recent closed event before the reopen
        // We reverse to start from the closest event to the reopen
        const previousCloseEvent = [...previousEvents].reverse().find(e =>
          e.event === 'closed'
        );

        console.log(`  Previous close event:`, previousCloseEvent);
        if (!previousCloseEvent) {
          console.log(`  No previous closed event found for issue #${issue.number}. Exiting...`);
          process.exit(1);
        }
        if (typeof previousCloseEvent.state_reason === 'undefined') {
          console.log(`  No state_reason found for issue #${issue.number}. Exiting...`);
          process.exit(1);
        }
        const previousState = previousCloseEvent.state_reason;


        console.log(`  Previous state before reopening: ${previousState}`);

        reopenedIssues.push({
          issue: issue,
          reopenedAt: lastReopenEvent.created_at,
          previousState: previousState
        });
      }
    } catch (error) {
      console.error(`Error fetching events for issue #${issue.number}:`, error.message);
    }
  }

  // Sort by most recently reopened
  reopenedIssues.sort((a, b) => new Date(b.reopenedAt) - new Date(a.reopenedAt));

  return reopenedIssues;
}

// Create readline interface for user input
function createReadlineInterface() {
  return readline.createInterface({
    input: process.stdin,
    output: process.stdout
  });
}

// Get user confirmation
function askForConfirmation(rl, question) {
  return new Promise((resolve) => {
    rl.question(question, (answer) => {
      resolve(answer.trim().toLowerCase());
    });
  });
}

// Close the reopened issues
async function closeReopenedIssues(octokit, issues) {
  if (issues.length === 0) {
    console.log('No reopened issues found.');
    return;
  }

  console.log(`Found ${issues.length} reopened issues to close.`);

  // Create readline interface for user input
  const rl = createReadlineInterface();

  // Flag to track if user wants to close all without prompting
  let closeAllWithoutPrompt = false;

  for (const item of issues) {
    const issue = item.issue;
    const previousState = item.previousState;

    console.log(`\nIssue #${issue.number}: ${issue.title}`);
    console.log(`  URL: ${issue.html_url}`);
    console.log(`  Reopened at: ${item.reopenedAt}`);
    console.log(`  Previous state: ${previousState}`);

    // If user hasn't opted to close all, ask for confirmation
    if (!closeAllWithoutPrompt) {
      let promptMessage = `Close this issue with state '${previousState}'? [y=yes, n=skip, o=open in browser first, a=yes to all] `;

      let answer = await askForConfirmation(rl, promptMessage);

      // Handle 'o' option to open in browser
      if (answer === 'o') {
        console.log(`  Opening issue #${issue.number} in browser...`);
        await open(issue.html_url);

        // After opening, ask again
        answer = await askForConfirmation(
          rl,
          `Close this issue with previous state '${previousState}'? [y=yes, n=skip, a=yes to all] `
        );
      }

      // Only check for 'o' in the first prompt, not in retries
      const validAnswers = ['y', 'n', 'a', ''];

      while (!validAnswers.includes(answer)) {
        let retryPrompt = `Invalid input. Close this issue with state '${previousState}'? [y=yes, n=skip, a=yes to all] `;

        answer = await askForConfirmation(rl, retryPrompt);

        // We don't offer browser opening during retries to simplify
      }

      if (answer === 'n') {
        console.log(`  Skipping issue #${issue.number}`);
        continue;
      } else if (answer === 'a') {
        closeAllWithoutPrompt = true;
        console.log('  Will close all remaining issues without further prompts');
      } else if (answer !== 'y' && answer !== '') {
        console.log(`  Invalid input '${answer}', should not happen, exiting...`);
        rl.close();
        process.exit(1);
      }
    }

    try {
      // Add a comment if configured
      if (CONFIG.closeComment) {
        await octokit.rest.issues.createComment({
          owner: CONFIG.owner,
          repo: CONFIG.repo,
          issue_number: issue.number,
          body: CONFIG.closeComment
        });

        console.log(`  Added comment: "${CONFIG.closeComment}"`);
      }

      // Close the issue with the same state it had before
      await octokit.rest.issues.update({
        owner: CONFIG.owner,
        repo: CONFIG.repo,
        issue_number: issue.number,
        state: 'closed',
        state_reason: previousState
      });

      console.log(`  ✓ Issue #${issue.number} closed successfully with state_reason: ${previousState}`);
    } catch (error) {
      console.error(`  ✗ Error closing issue #${issue.number}:`, error.message);
    }
  }

  // Close the readline interface
  rl.close();

  console.log(`\nCompleted processing ${issues.length} reopened issues.`);
}

// Main function
async function main() {
  try {
    const octokit = initializeGitHubClient();

    const reopenedIssues = await findReopenedIssues(octokit);

    if (!CONFIG.dryRun) {
      await closeReopenedIssues(octokit, reopenedIssues);
    } else {
      console.log('DRY RUN: The following issues would be closed:');
      console.log(reopenedIssues);
    }

  } catch (error) {
    console.error('Error:', error.message);
    process.exit(1);
  }
}

// Run the script
main();
