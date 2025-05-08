package checker

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gjermundgaraba/changelog-checker/pkg/db"
	"github.com/gjermundgaraba/changelog-checker/pkg/github"
	"github.com/gjermundgaraba/changelog-checker/pkg/types"
)

// Checker checks changelog entries against GitHub PR info
type Checker struct {
	githubClient *github.Client
	openAIClient *OpenAIClient
	db           *db.DB
	repoOwner    string
	repoName     string
	verbose      bool
}

// NewChecker creates a new changelog checker
func NewChecker(githubClient *github.Client, openAIKey, repoOwner, repoName string, database *db.DB, verbose bool) *Checker {
	var openAIClient *OpenAIClient
	if openAIKey != "" {
		openAIClient = NewOpenAIClient(openAIKey)
	}

	return &Checker{
		githubClient: githubClient,
		openAIClient: openAIClient,
		db:           database,
		repoOwner:    repoOwner,
		repoName:     repoName,
		verbose:      verbose,
	}
}

// ExtractPRNumbers extracts PR numbers from a changelog section
func (c *Checker) ExtractPRNumbers(changelogSection string) []int {
	var prNumbers []int

	// Count the lines that start with '*' to get total entries
	starLineCount := 0
	entryWithoutPR := 0
	scanner := bufio.NewScanner(strings.NewReader(changelogSection))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "*") {
			starLineCount++
			if !strings.Contains(line, "[\\#") {
				entryWithoutPR++
				if c.verbose {
					log.Printf("Entry without PR number: %s", line)
				}
			}
		}
	}
	log.Printf("Found %d changelog entries, entries without PR: %d", starLineCount, entryWithoutPR)

	// Extract PR numbers using multiple patterns

	// Pattern 1: Standard PR references: [\#123]
	re := regexp.MustCompile(`\[\\#(\d+)\]`)
	matches := re.FindAllStringSubmatch(changelogSection, -1)

	for _, match := range matches {
		if len(match) > 1 {
			number, err := strconv.Atoi(match[1])
			if err != nil {
				continue
			}

			// Check if we've already seen this PR number
			found := false
			for _, pr := range prNumbers {
				if pr == number {
					found = true
					break
				}
			}

			if !found {
				prNumbers = append(prNumbers, number)
			}
		}
	}

	// Debug output to see distribution of PR numbers
	scanner = bufio.NewScanner(strings.NewReader(changelogSection))
	lineNum := 0
	multiPRLine := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.HasPrefix(line, "*") {
			// Count PR references in this line
			matches := re.FindAllStringSubmatch(line, -1)
			if len(matches) > 1 {
				multiPRLine++
				if c.verbose {
					log.Printf("Line %d has multiple PR numbers: %s", lineNum, line)
				}
			}
		}
	}
	if c.verbose {
		log.Printf("Lines with multiple PR numbers: %d", multiPRLine)
	}

	return prNumbers
}

// findSection checks if a section with the given header exists
func findSection(scanner *bufio.Scanner, header string) bool {
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), header) {
			return true
		}
	}
	return false
}

// GetChangelogSection extracts the changelog section for a specific version
func (c *Checker) GetChangelogSection(changelogFile, versionTag string) (string, error) {
	file, err := os.Open(changelogFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var sectionLines []string
	inSection := false

	// If no version specified, first try "Unreleased" then fall back to finding the latest version
	if versionTag == "" {
		// Start with "Unreleased"
		versionTag = "Unreleased"

		// If we need to extract the latest version
		if !findSection(scanner, fmt.Sprintf("## [%s]", versionTag)) {
			// Reset the scanner to start of file
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)

			// Find the first version section (which is the latest version)
			for scanner.Scan() {
				line := scanner.Text()
				if match := regexp.MustCompile(`^## \[(v\d+\.\d+\.\d+)\]`).FindStringSubmatch(line); len(match) > 1 {
					versionTag = match[1]
					break
				}
			}

			// Reset scanner again
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
		} else {
			// Reset the scanner since we've consumed lines
			file.Seek(0, 0)
			scanner = bufio.NewScanner(file)
		}
	}

	// Handle both formats: ## [v1.0.0] and ## [Unreleased]
	versionHeader := fmt.Sprintf("## [%s]", versionTag)
	if !strings.HasPrefix(versionTag, "v") && versionTag != "Unreleased" {
		versionHeader = fmt.Sprintf("## [v%s]", versionTag)
	}

	for scanner.Scan() {
		line := scanner.Text()

		// Start of our section
		if strings.HasPrefix(line, versionHeader) {
			inSection = true
			sectionLines = append(sectionLines, line)
			continue
		}

		// End of our section (new version section starts)
		if inSection && strings.HasPrefix(line, "## [") {
			break
		}

		// Add lines while we're in our section
		if inSection {
			sectionLines = append(sectionLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	if len(sectionLines) == 0 {
		return "", fmt.Errorf("no section found for %s in changelog file", versionTag)
	}

	return strings.Join(sectionLines, "\n"), nil
}

// GetPRDescriptionFromLine extracts the PR description from a changelog line
func (c *Checker) GetPRDescriptionFromLine(line string, prNumber int) string {
	// Look for the PR number in the line
	prRef := fmt.Sprintf("[\\#%d]", prNumber)
	if !strings.Contains(line, prRef) {
		return ""
	}

	// Format: * (component) [\#PR](url) Description
	if match := regexp.MustCompile(`^\* \([^)]*\) \[\\#\d+\]\([^)]+\) (.+)$`).FindStringSubmatch(line); len(match) > 1 {
		return match[1]
	}

	// Format: * [\#PR](url) Description
	if match := regexp.MustCompile(`^\* \[\\#\d+\]\([^)]+\) (.+)$`).FindStringSubmatch(line); len(match) > 1 {
		return match[1]
	}

	// Try a more general approach
	if match := regexp.MustCompile(`\[\\#\d+\]\([^)]+\) (.+)$`).FindStringSubmatch(line); len(match) > 1 {
		return match[1]
	}

	return ""
}

// CheckSimilarity checks similarity between changelog description and PR title
func (c *Checker) CheckSimilarity(changelogDesc, prTitle string) types.PRStatus {
	// Simple similarity check
	changelogLower := strings.ToLower(strings.ReplaceAll(changelogDesc, "`", ""))
	prTitleLower := strings.ToLower(prTitle)

	// Strip punctuation at the end
	if idx := strings.Index(changelogLower, "."); idx != -1 {
		changelogLower = changelogLower[:idx]
	}

	if strings.Contains(prTitleLower, changelogLower) || strings.Contains(changelogLower, prTitleLower) {
		return types.StatusGoodMatch
	}

	// Try OpenAI similarity check if client is available
	if c.openAIClient != nil {
		similar, err := c.openAIClient.CheckSimilarity(prTitle, changelogDesc)
		if err != nil {
			if c.verbose {
				log.Printf("OpenAI similarity check error: %v", err)
			}
		} else if similar {
			return types.StatusGoodMatch
		}
	}

	return types.StatusPotentialMismatch
}

// FindPRLineInSection finds the line containing a PR in the changelog section
func (c *Checker) FindPRLineInSection(prNumber int, section string) string {
	scanner := bufio.NewScanner(strings.NewReader(section))
	prRef := fmt.Sprintf("[\\#%d]", prNumber)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, prRef) {
			return line
		}
	}

	return ""
}

// CheckPR checks a single PR
func (c *Checker) CheckPR(prNumber int, changelogSection string) types.PRResult {
	result := types.PRResult{
		Number: prNumber,
	}

	// Find the PR line in the changelog
	line := c.FindPRLineInSection(prNumber, changelogSection)
	if line == "" {
		result.Status = types.StatusNotFound
		result.Error = fmt.Errorf("PR #%d not found in changelog section", prNumber)
		return result
	}

	// Extract changelog description
	result.ChangelogDesc = c.GetPRDescriptionFromLine(line, prNumber)
	if result.ChangelogDesc == "" {
		result.Status = types.StatusNotFound
		result.Error = fmt.Errorf("couldn't extract description for PR #%d", prNumber)
		return result
	}

	// Check validation cache first
	if c.db != nil {
		status, found, err := c.db.GetValidationResult(c.repoOwner, c.repoName, prNumber, result.ChangelogDesc)
		if err != nil {
			if c.verbose {
				log.Printf("Error checking validation cache: %v", err)
			}
		} else if found {
			// Use cached validation result
			if c.verbose {
				log.Printf("Using cached validation result for PR #%d", prNumber)
			}
			result.Status = types.PRStatus(status)

			// Still need to get the PR title for display purposes
			prTitle, err := c.githubClient.GetPRInfo(c.repoOwner, c.repoName, prNumber)
			if err != nil {
				result.Error = err
			} else {
				result.PRTitle = prTitle
			}

			return result
		}
	}

	// Cache miss or error - get PR title from GitHub API
	prTitle, err := c.githubClient.GetPRInfo(c.repoOwner, c.repoName, prNumber)
	if err != nil {
		result.Status = types.StatusNotFound
		result.Error = err
		return result
	}

	result.PRTitle = prTitle

	// Check similarity
	result.Status = c.CheckSimilarity(result.ChangelogDesc, prTitle)

	// Store the validation result in cache
	if c.db != nil {
		if err := c.db.StoreValidationResult(c.repoOwner, c.repoName, prNumber, result.ChangelogDesc, int(result.Status)); err != nil {
			if c.verbose {
				log.Printf("Error caching validation result: %v", err)
			}
		}
	}

	return result
}

// CheckChangelog checks changelog entries against GitHub PR info
// It returns the list of PRs found along with their validation status
func (c *Checker) CheckChangelog(changelogFile, versionTag string, limit int) ([]types.PRResult, error) {
	if c.verbose {
		log.Printf("Checking Unreleased changelog entries...")
	}
	// Get the changelog section for the specified version
	section, err := c.GetChangelogSection(changelogFile, versionTag)
	if err != nil {
		return nil, err
	}

	// Extract PR numbers from the section
	prNumbers := c.ExtractPRNumbers(section)
	if len(prNumbers) == 0 {
		return nil, fmt.Errorf("no PR numbers found in the changelog section")
	}

	if c.verbose {
		log.Printf("Found %d unique PR numbers in the changelog", len(prNumbers))
		// Debug: Print all PR numbers
		log.Printf("Found the following PR numbers in Unreleased section: %v", prNumbers)
	}

	// Apply limit if specified
	if limit > 0 && limit < len(prNumbers) {
		if c.verbose {
			log.Printf("Limiting to %d PRs (test mode)", limit)
		}
		if limit == 3 {
			// For testing, grab first, middle and last PR
			middle := len(prNumbers) / 2
			prNumbers = []int{prNumbers[0], prNumbers[middle], prNumbers[len(prNumbers)-1]}
		} else {
			prNumbers = prNumbers[:limit]
		}
	}

	// Check each PR
	var results []types.PRResult
	for _, prNumber := range prNumbers {
		result := c.CheckPR(prNumber, section)
		results = append(results, result)
	}

	return results, nil
}

