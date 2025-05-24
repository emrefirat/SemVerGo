package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

// appVersion holds the current version of the application.
// This should be updated manually for each release or automated via build scripts.
var appVersion = "v0.1.0-beta.0" // Current pre-release version

// GitConfig represents required Git configuration
var requiredGitConfigs = []string{
	"user.name",
	"user.email",
}

// commitPattern follows the Conventional Commits specification
// See: https://www.conventionalcommits.org/
var commitPattern = regexp.MustCompile(`^(?P<type>feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(?:\((?P<scope>[^()\r\n]*)\)|\()?(?P<breaking>!)?: (?P<subject>.*)$`)

// validateCommitMessage checks if a commit message follows the Conventional Commits spec
func validateCommitMessage(message string) (bool, string) {
	// Skip merge commits
	if strings.HasPrefix(message, "Merge ") {
		return true, ""
	}

	// Check if message matches the pattern
	if !commitPattern.MatchString(message) {
		errMsg := fmt.Sprintf(`
Invalid commit message format: "%s"

Please follow the Conventional Commits specification:
<type>[optional scope]: <description>

Available types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert

Example: feat(auth): add login functionality`, strings.TrimSpace(message))
		return false, errMsg
	}
	return true, ""
}

func validateGitConfig() error {
	for _, config := range requiredGitConfigs {
		out, err := exec.Command("git", "config", "--get", config).Output()
		if err != nil {
			return fmt.Errorf("error getting Git config %s: %v", config, err)
		}
		if strings.TrimSpace(string(out)) == "" {
			return fmt.Errorf("Git config %s is not set. Please set it with: git config --global %s 'Your Value'", config, config)
		}
	}
	return nil
}

func checkGitStatus() error {
	// Check for uncommitted changes
	statusCmd := exec.Command("git", "status", "--porcelain")
	output, err := statusCmd.Output()
	if err != nil {
		return fmt.Errorf("error checking git status: %v", err)
	}

	if len(strings.TrimSpace(string(output))) > 0 {
		return fmt.Errorf("working directory is not clean. Please commit or stash your changes first")
	}

	// Check if branch is tracking a remote and is up to date
	trackingBranchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err := trackingBranchCmd.Run(); err == nil {
		// We have a tracking branch, check if it's up to date
		diffCmd := exec.Command("git", "diff", "--name-only", "HEAD", "@{u}")
		diffOutput, err := diffCmd.Output()
		if err != nil {
			// If we can't check remote status, just log a warning but don't fail
			fmt.Printf("Warning: Could not check remote status: %v\n", err)
		} else if len(strings.TrimSpace(string(diffOutput))) > 0 {
			// Only warn about being behind remote, don't fail the operation
			fmt.Printf("Warning: Your branch is behind the remote. Consider pulling the latest changes.\n")
		}
	} else {
		// No tracking branch, which is fine for local branches
		fmt.Println("No remote tracking branch found. This is normal for local branches.")
	}

	return nil
}

// isGitRepository checks if the current directory is a Git repository.
func isGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	err := cmd.Run()
	return err == nil
}

func main() {
	// Define flags
	showAppVersion := flag.Bool("version", false, "Display the application's version.")
	gitDir := flag.String("git-address", ".", "Path to git repository (default: current directory)")
	branch := flag.String("branch", "", "Branch name (default: current branch)")
	preRelease := flag.Bool("preRelease", false, "Enable pre-release versioning based on branch name")
	ciMode := flag.Bool("ci", false, "Run in CI mode (auto-detect branch, auto-push tags)")
	pushBranch := flag.Bool("push-branch", false, "Push the branch to remote if it doesn't exist or is behind")
	setVersionFlag := flag.String("set-version", "", "Specify the exact version to be released (e.g., 1.2.3) to override automatic versioning.")
	skipChecks := flag.Bool("skip-checks", false, "Skip git configuration and status checks (use with caution)")
	nextVersionOnly := flag.Bool("next-version-only", false, "Only display the next version, do not create a tag")
	tagFormat := flag.String("tag-format", "v{{.Major}}.{{.Minor}}.{{.Patch}}{{.Prerelease}}", "Custom format for the git tag. Placeholders: {{.Major}}, {{.Minor}}, {{.Patch}}, {{.Prerelease}} (includes leading hyphen if present, e.g., '-beta.1'). Example: 'v{{.Major}}.{{.Minor}}.{{.Patch}}{{.Prerelease}}' or 'release-{{.Major}}.{{.Minor}}.{{.Patch}}'")
	debugMode := flag.Bool("debug", false, "Enable debug output for verbose logging")
	outputChangelogEnabled := flag.Bool("output-changelog", false, "Enable generation of CHANGELOG.md file. Defaults to false.")
	dryRun := flag.Bool("dry-run", false, "Perform a dry run, showing what would happen without making changes.")

	// Parse flags
	flag.Parse()

	// Handle --version flag immediately if present
	if *showAppVersion {
		fmt.Println(appVersion)
		os.Exit(0)
	}

	if *dryRun {
		fmt.Println("Dry run mode enabled: No actual changes will be made to the Git repository or files.")
	}

	if *debugMode {
		fmt.Printf("DEBUG: Raw tagFormat flag value: '%s'\n", *tagFormat)
	}

	absGitDir, err := filepath.Abs(*gitDir)
	if err != nil {
		fmt.Printf("Error getting absolute path: %v\n", err)
		os.Exit(1)
	}

	if err := os.Chdir(absGitDir); err != nil {
		fmt.Printf("Error changing to git directory: %v\n", err)
		os.Exit(1)
	}

	if !isGitRepository() {
		fmt.Printf("Error: '%s' is not a Git repository.\n", absGitDir)
		os.Exit(1)
	}

	if !*skipChecks {
		if err := validateGitConfig(); err != nil {
			fmt.Printf("Git configuration error: %v\n", err)
			os.Exit(1)
		}

		if err := checkGitStatus(); err != nil {
			fmt.Printf("Git status check failed: %v\n", err)
			os.Exit(1)
		}
	} else if *ciMode {
		fmt.Println("Skipping Git checks in CI mode.")
	}

	if *branch == "" {
		out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			fmt.Printf("Error getting current branch: %v\n", err)
			os.Exit(1)
		}
		*branch = strings.TrimSpace(string(out))
	}

	if !*preRelease && !isDefaultBranch(*branch) {
		*preRelease = true
		fmt.Printf("Auto-enabled pre-release for non-default branch: %s\n", *branch)
	} else {
		fmt.Printf("Pre-release mode: %v\n", *preRelease)
	}

	if *ciMode {
		fmt.Printf("CI Mode: Branch: %s, Pre-release: %v\n", *branch, *preRelease)
	}

	// Get current version to determine the commit range for analysis
	currentVersionForCommits, err := getCurrentVersion()
	if err != nil {
		fmt.Printf("Error getting current version for commit analysis: %v\n", err)
		os.Exit(1)
	}
	var fromRef string
	if currentVersionForCommits.Major() == 0 && currentVersionForCommits.Minor() == 0 && currentVersionForCommits.Patch() == 0 && currentVersionForCommits.Prerelease() == "" {
		fromRef = "" // If starting from 0.0.0, get all commits
	} else {
		fromRef = "v" + currentVersionForCommits.String()
	}

	commitMessages, err := getCommitMessagesBetweenRefs(fromRef, "HEAD")
	if err != nil {
		fmt.Printf("Error getting commit messages for analysis: %v\n", err)
		os.Exit(1)
	}

	if *debugMode {
		fmt.Printf("DEBUG: Commit messages for bump type analysis (from %s to HEAD):\n", fromRef)
		for i, msg := range commitMessages {
			fmt.Printf("  - %d: '%s'\n", i, msg)
		}
	}

	// Validate the latest commit message for format, but determine bump type from all relevant commits
	latestCommitMsgBytes, err := exec.Command("git", "log", "-1", "--pretty=%B").Output()
	if err != nil {
		fmt.Printf("Error getting latest commit message for validation: %v\n", err)
		os.Exit(1)
	}
	latestCommitMsg := strings.TrimSpace(string(latestCommitMsgBytes))

	if ok, errMsg := validateCommitMessage(latestCommitMsg); !ok {
		fmt.Printf("Invalid latest commit message: %s\n", errMsg)
		os.Exit(1)
	}

	bumpType, err := determineBumpType(commitMessages) // Pass all relevant commit messages
	if err != nil {
		fmt.Printf("Error determining version bump type: %v\n", err)
		os.Exit(1)
	}

	if bumpType == "none" {
		fmt.Println("No version bump needed based on commit history.")
		os.Exit(0)
	}
	
	fmt.Printf("Valid commit message: %s\n", latestCommitMsg) // Still show the latest commit message
	fmt.Printf("Based on commit history, will perform %s version bump.\n", bumpType)

	var newVersion string
	var currentVersion *semver.Version // Store current version for changelog generation

	// Re-fetch current version if needed, or use currentVersionForCommits
	currentVersion = currentVersionForCommits // Use the version fetched earlier for consistency

	if *setVersionFlag != "" { // Use the new setVersionFlag
		versionStr := *setVersionFlag
		if !strings.HasPrefix(versionStr, "v") {
			versionStr = "v" + versionStr
		}

		if tagExists(versionStr) {
			fmt.Printf("Error: Version %s already exists as a tag.\n", versionStr)
			os.Exit(1)
		}

		_, verErr := semver.NewVersion(strings.TrimPrefix(versionStr, "v"))
		if verErr != nil {
			fmt.Printf("Error: Invalid version format: %v\n", verErr)
			os.Exit(1)
		}

		newVersion = strings.TrimPrefix(versionStr, "v")
	} else {
		newVersion, err = calculateNewVersion(currentVersion, bumpType, *branch, *preRelease)
		if err != nil {
			fmt.Printf("Error calculating new version: %v\n", err)
			os.Exit(1)
		}
	}

	if *debugMode {
		fmt.Printf("DEBUG: Calculated newVersion string: '%s'\n", newVersion)
	}

	var finalTagName string
	if *tagFormat != "" {
		parsedNewVer, err := semver.NewVersion(newVersion)
		if err != nil {
			fmt.Printf("Error parsing calculated new version '%s' for formatting: %v\n", newVersion, err)
			os.Exit(1)
		}

		if *debugMode {
			fmt.Printf("DEBUG: Parsed New Version: %s\n", parsedNewVer.String())
			fmt.Printf("DEBUG: Major: %d, Minor: %d, Patch: %d, Prerelease: '%s'\n",
				parsedNewVer.Major(), parsedNewVer.Minor(), parsedNewVer.Patch(), parsedNewVer.Prerelease())
		}

		formattedTag := *tagFormat
		if *debugMode {
			fmt.Printf("DEBUG: Initial formattedTag for replacements: '%s'\n", formattedTag)
		}

		formattedTag = strings.ReplaceAll(formattedTag, "{{.Major}}", strconv.FormatUint(parsedNewVer.Major(), 10))
		if *debugMode {
			fmt.Printf("DEBUG: formattedTag after {{.Major}}: '%s'\n", formattedTag)
		}

		formattedTag = strings.ReplaceAll(formattedTag, "{{.Minor}}", strconv.FormatUint(parsedNewVer.Minor(), 10))
		if *debugMode {
			fmt.Printf("DEBUG: formattedTag after {{.Minor}}: '%s'\n", formattedTag)
		}

		formattedTag = strings.ReplaceAll(formattedTag, "{{.Patch}}", strconv.FormatUint(parsedNewVer.Patch(), 10))
		if *debugMode {
			fmt.Printf("DEBUG: formattedTag after {{.Patch}}: '%s'\n", formattedTag)
		}

		prereleasePart := parsedNewVer.Prerelease()
		if prereleasePart != "" {
			formattedTag = strings.ReplaceAll(formattedTag, "{{.Prerelease}}", "-"+prereleasePart)
			if *debugMode {
				fmt.Printf("DEBUG: formattedTag after {{.Prerelease}} (with value): '%s'\n", formattedTag)
			}
		} else {
			formattedTag = strings.ReplaceAll(formattedTag, "{{.Prerelease}}", "")
			if *debugMode {
				fmt.Printf("DEBUG: formattedTag after {{.Prerelease}} (empty value): '%s'\n", formattedTag)
			}
		}
		finalTagName = formattedTag
	} else {
		finalTagName = "v" + newVersion
	}

	// If next-version-only flag is set, just print the version and exit
	if *nextVersionOnly {
		fmt.Println(finalTagName)
		os.Exit(0)
	}

	changelogPath := "CHANGELOG.md" // The fixed path for the changelog
	changelogGenerated := false     // Flag to track if changelog was actually generated

	// Generate Release Notes if --output-changelog is enabled AND not in pre-release mode
	// Release notes are typically generated for final releases, not pre-releases.
	if *outputChangelogEnabled && !*preRelease { // Check the boolean flag
		// Use the previous release tag as the 'from' tag for changelog generation
		// and "HEAD" as the 'to' reference, as the new tag might not exist yet.
		oldTag := "v" + currentVersion.String() // Assuming currentVersion is the last *release* tag
		if currentVersion.Major() == 0 && currentVersion.Minor() == 0 && currentVersion.Patch() == 0 && currentVersion.Prerelease() == "" {
			// If starting from 0.0.0, use an empty string for the 'from' tag to get all commits
			oldTag = ""
		}
		
		fmt.Printf("Generating release notes from %s to %s (HEAD)...\n", oldTag, finalTagName) // Log finalTagName for clarity
		if *dryRun {
			fmt.Printf("[DRY-RUN] Would generate release notes to: %s\n", changelogPath) // Hardcode for dry-run message
		} else {
			if err := generateReleaseNotes(oldTag, finalTagName, changelogPath, *debugMode); err != nil { // Hardcode for actual write
				fmt.Printf("Error generating release notes: %v\n", err)
				// Don't exit, allow tag creation to proceed even if notes fail
			} else {
				fmt.Printf("Release notes generated and saved to %s\n", changelogPath)
				changelogGenerated = true // Set flag if successful
			}
		}
	}

	// Add and Commit Changelog if it was generated and not in dry-run mode
	if changelogGenerated && !*dryRun {
		fmt.Printf("Committing %s...\n", changelogPath)
		if err := addAndCommitChangelog(changelogPath, finalTagName); err != nil {
			fmt.Printf("Error committing changelog: %v\n", err)
			os.Exit(1) // This is a critical step, so exit on failure
		}
		fmt.Printf("Changelog %s committed.\n", changelogPath)
	} else if changelogGenerated && *dryRun {
		fmt.Printf("[DRY-RUN] Would commit %s with message 'chore(release): update changelog for %s [skip-ci]'.\n", changelogPath, finalTagName)
	}

	// Create git tag
	fmt.Printf("Creating tag: %s\n", finalTagName)
	if *dryRun {
		fmt.Printf("[DRY-RUN] Would create tag: %s\n", finalTagName)
	} else {
		if err := createGitTag(finalTagName); err != nil {
			fmt.Printf("Error creating git tag: %v\n", err)
			os.Exit(1)
		}
	}

	// Push tag if in CI mode or push-branch is enabled
	if *ciMode || *pushBranch {
		if *dryRun {
			fmt.Printf("[DRY-RUN] Would push tag: %s\n", finalTagName)
			if *pushBranch {
				fmt.Println("[DRY-RUN] Would also push the current branch.")
			}
		} else {
			if err := pushTag(finalTagName); err != nil {
				fmt.Printf("Error pushing tag: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Successfully created and pushed version: %s\n", finalTagName)

			// Push the branch if push-branch is enabled
			if err := pushCurrentBranch(); err != nil {
				fmt.Printf("Warning: Could not push branch: %v\n", err)
			} else {
				fmt.Printf("Successfully pushed branch to remote.\n")
			}
		}
	} else {
		fmt.Printf("New version created: %s\n", finalTagName)
		fmt.Printf("Run 'git push origin %s' to push the tag to remote.\n", finalTagName)
	}
}

// getCommitMessagesBetweenRefs gets commit messages between two Git references (tags, branches, SHAs)
func getCommitMessagesBetweenRefs(fromRef, toRef string) ([]string, error) {
	var commitRange string
	if fromRef == "" {
		// If fromRef is empty, get all commits up to toRef
		commitRange = toRef
	} else {
		commitRange = fmt.Sprintf("%s..%s", fromRef, toRef)
	}

	logCmd := exec.Command("git", "log", "--format=%B", commitRange)
	out, err := logCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("error getting commit messages for range %s: %v\nOutput: %s", commitRange, err, string(out))
	}

	messages := strings.Split(string(out), "\n\n")
	var commitMsgs []string
	for _, msg := range messages {
		msg = strings.TrimSpace(msg)
		if msg != "" {
			commitMsgs = append(commitMsgs, msg)
		}
	}
	return commitMsgs, nil
}

// determineBumpType analyzes a list of commit messages to determine the version bump type
func determineBumpType(commitMsgs []string) (string, error) {
	bumpType := "none" // Default to 'none' if no version-bumping commits are found

	for _, msg := range commitMsgs {
		msg = strings.TrimSpace(msg)
		if msg == "" || strings.HasPrefix(msg, "Merge ") {
			continue
		}

		matches := commitPattern.FindStringSubmatch(msg)
		if len(matches) == 0 {
			// If it doesn't match conventional commits, it's an invalid format for version bumping.
			// We skip it for bump type determination, assuming validateCommitMessage handles format validation.
			continue
		}

		commitType := matches[commitPattern.SubexpIndex("type")]
		isBreaking := matches[commitPattern.SubexpIndex("breaking")] != "" || strings.Contains(msg, "BREAKING CHANGE:")

		if isBreaking {
			return "major", nil // Breaking change takes highest precedence, immediately return major
		}

		if commitType == "feat" {
			if bumpType != "major" { // Only set to minor if not already major
				bumpType = "minor"
			}
		} else if commitType == "fix" {
			if bumpType != "major" && bumpType != "minor" { // Only set to patch if not already major or minor
				bumpType = "patch"
			}
		}
		// For other commit types (docs, style, refactor, perf, test, build, ci, chore, revert),
		// if no 'major', 'minor', or 'patch' has been determined yet, 'bumpType' remains 'none'.
	}

	return bumpType, nil
}

// generateReleaseNotes creates Markdown formatted release notes
func generateReleaseNotes(oldTag, newTagForHeader, outputPath string, debugMode bool) error {
	// Use "HEAD" as the 'toRef' for git log, as newTagForHeader might not exist yet as a tag
	commitMsgs, err := getCommitMessagesBetweenRefs(oldTag, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get commit messages for release notes: %v", err)
	}

	if debugMode {
		fmt.Printf("DEBUG: Commits for release notes (%s..HEAD):\n", oldTag)
		for i, msg := range commitMsgs {
			fmt.Printf("  %d: '%s'\n", i, msg)
		}
	}

	// Categorize commits
	breakingChanges := []string{}
	features := []string{}
	bugFixes := []string{}
	otherChanges := []string{}

	for _, msg := range commitMsgs {
		if shouldSkipCI(msg) || strings.HasPrefix(msg, "Merge ") {
			continue // Skip commits that should not be in release notes
		}

		matches := commitPattern.FindStringSubmatch(msg)
		if len(matches) == 0 {
			// If it doesn't match conventional commits, add to other changes
			otherChanges = append(otherChanges, strings.Split(msg, "\n")[0]) // Just take the subject line
			continue
		}

		commitType := matches[commitPattern.SubexpIndex("type")]
		commitSubject := matches[commitPattern.SubexpIndex("subject")]
		isBreaking := matches[commitPattern.SubexpIndex("breaking")] != "" || strings.Contains(msg, "BREAKING CHANGE:")

		if isBreaking {
			// Extract full breaking change message if present in body
			breakingMsg := fmt.Sprintf("- **BREAKING CHANGE:** %s", commitSubject)
			if strings.Contains(msg, "BREAKING CHANGE:") {
				parts := strings.SplitN(msg, "BREAKING CHANGE:", 2)
				if len(parts) > 1 {
					breakingMsg += "\n  " + strings.TrimSpace(parts[1])
				}
			}
			breakingChanges = append(breakingChanges, breakingMsg)
		}

		// Only add to categories if not a breaking change (to avoid duplication)
		if !isBreaking {
			switch commitType {
			case "feat":
				features = append(features, fmt.Sprintf("- **feat:** %s", commitSubject))
			case "fix":
				bugFixes = append(bugFixes, fmt.Sprintf("- **fix:** %s", commitSubject))
			default:
				// Include other types that might be relevant for a changelog, but not major/minor/patch bumps
				if commitType != "docs" && commitType != "style" && commitType != "test" && commitType != "chore" {
					otherChanges = append(otherChanges, fmt.Sprintf("- **%s:** %s", commitType, commitSubject))
				}
			}
		}
	}

	// Build Markdown content
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## %s (%s)\n\n", newTagForHeader, time.Now().Format("2006-01-02")))

	if len(breakingChanges) > 0 {
		sb.WriteString("### BREAKING CHANGES\n\n")
		for _, change := range breakingChanges {
			sb.WriteString(change + "\n")
		}
		sb.WriteString("\n")
	}

	if len(features) > 0 {
		sb.WriteString("### Features\n\n")
		for _, feature := range features {
			sb.WriteString(feature + "\n")
		}
		sb.WriteString("\n")
	}

	if len(bugFixes) > 0 {
		sb.WriteString("### Bug Fixes\n\n")
		for _, fix := range bugFixes {
			sb.WriteString(fix + "\n")
		}
		sb.WriteString("\n")
	}

	if len(otherChanges) > 0 {
		sb.WriteString("### Other Changes\n\n")
		for _, other := range otherChanges {
			sb.WriteString(other + "\n")
		}
		sb.WriteString("\n")
	}

	newChangelogContent := sb.String()

	// Read existing changelog content if file exists
	var existingContent []byte
	if _, err := os.Stat(outputPath); err == nil { // File exists
		existingContent, err = ioutil.ReadFile(outputPath)
		if err != nil {
			return fmt.Errorf("failed to read existing changelog file '%s': %v", outputPath, err)
		}
	}

	// Prepend new content to existing content
	finalContent := []byte(newChangelogContent + string(existingContent))

	// Write to file
	if err := ioutil.WriteFile(outputPath, finalContent, 0644); err != nil {
		return fmt.Errorf("failed to write changelog to file '%s': %v", outputPath, err)
	}

	return nil
}

func getCurrentVersion() (*semver.Version, error) {
	// Get all tags
	out, err := exec.Command("git", "tag", "-l", "v*", "--sort=-v:refname").Output()
	if err != nil {
		return nil, fmt.Errorf("error getting git tags: %v", err)
	}

	tags := strings.Fields(string(out))

	// Find the highest version tag
	var latestVersion *semver.Version
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "v") {
			verStr := strings.TrimPrefix(tag, "v")
			v, err := semver.NewVersion(verStr)
			if err == nil {
				if latestVersion == nil || v.GreaterThan(latestVersion) {
					latestVersion = v
				}
			}
		}
	}

	if latestVersion != nil {
		return latestVersion, nil
	}

	// If no valid version tags found, start with 0.0.0
	return semver.NewVersion("0.0.0")
}

func calculateNewVersion(current *semver.Version, bumpType, branch string, preRelease bool) (string, error) {
	if !preRelease {
		// If not a pre-release, simply increment the current version based on bumpType
		switch bumpType {
		case "major":
			return current.IncMajor().String(), nil
		case "minor":
			return current.IncMinor().String(), nil
		default: // patch
			return current.IncPatch().String(), nil
		}
	}

	// Handle pre-release versioning
	sanitizedBranch := regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(branch, "-")

	// 1. Find the highest pre-release tag belonging to the current branch
	branchPreReleasePattern := regexp.QuoteMeta("v") + `\d+\.\d+\.\d+-` + regexp.QuoteMeta(sanitizedBranch) + `\.(\d+)$`
	branchPreReleaseRegex := regexp.MustCompile(branchPreReleasePattern)
	var highestBranchPreRelease *semver.Version
	var highestBranchPreReleaseNum = -1

	out, err := exec.Command("git", "tag", "-l", "--sort=-v:refname").Output()
	if err == nil {
		for _, tag := range strings.Fields(string(out)) {
			matches := branchPreReleaseRegex.FindStringSubmatch(tag)
			if len(matches) > 1 {
				num, err := strconv.Atoi(matches[1])
				if err == nil {
					v, err := semver.NewVersion(strings.TrimPrefix(tag, "v"))
					if err == nil && num > highestBranchPreReleaseNum {
						highestBranchPreRelease = v
						highestBranchPreReleaseNum = num
					}
				}
			}
		}
	}

	if highestBranchPreRelease != nil {
		// If a branch-specific pre-release tag was found, increment its pre-release number
		newPreRelease := fmt.Sprintf("%s.%d", sanitizedBranch, highestBranchPreReleaseNum+1)
		newVer := *highestBranchPreRelease // Start with the found highest pre-release version

		// Apply the bump type to the base version (major.minor.patch) of the highest pre-release
		// and then re-apply the new pre-release suffix.
		var bumpedBase semver.Version
		switch bumpType {
		case "major":
			bumpedBase = newVer.IncMajor()
		case "minor":
			bumpedBase = newVer.IncMinor()
		case "patch":
			bumpedBase = newVer.IncPatch()
		default:
			// If bumpType is not major/minor/patch, just take the base of current pre-release
			// and attach the incremented pre-release suffix.
			base := fmt.Sprintf("%d.%d.%d", newVer.Major(), newVer.Minor(), newVer.Patch())
			v, _ := semver.NewVersion(base)
			bumpedBase = *v
		}
		newVer, _ = bumpedBase.SetPrerelease(newPreRelease)
		return newVer.String(), nil
	} else {
		// No pre-release tags found for this branch.
		// Get the latest *release* version (non-pre-release) from the entire repository.
		latestRelease, err := getCurrentReleaseVersion()
		if err != nil {
			latestRelease, _ = semver.NewVersion("0.0.0") // Fallback if no release versions found
		}

		// Create the first pre-release for this branch based on the latest release version.
		// We do NOT apply the bumpType here, as the first pre-release should be based on the
		// exact release version it branched off from (e.g., v0.2.0-demo.0 if branched from v0.2.0).
		newPreRelease := fmt.Sprintf("%s.0", sanitizedBranch)
		newVer, _ := latestRelease.SetPrerelease(newPreRelease)
		return newVer.String(), nil
	}
}

// getCurrentReleaseVersion finds the highest non-pre-release tag.
func getCurrentReleaseVersion() (*semver.Version, error) {
	out, err := exec.Command("git", "tag", "-l", "v*", "--sort=-v:refname").Output()
	if err != nil {
		return nil, fmt.Errorf("error getting git tags: %v", err)
	}

	tags := strings.Fields(string(out))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "v") {
			verStr := strings.TrimPrefix(tag, "v")
			v, err := semver.NewVersion(verStr)
			if err == nil && v.Prerelease() == "" { // Only consider release versions (no pre-release suffix)
				return v, nil
			}
		}
	}
	// If no valid release tags found, start with 0.0.0
	v, _ := semver.NewVersion("0.0.0")
	return v, nil
}

// isDefaultBranch checks if the given branch is the default branch of the repository
func isDefaultBranch(branch string) bool {
	// First try to get the default branch from remote
	cmd := exec.Command("git", "remote", "show", "origin")
	out, err := cmd.CombinedOutput()
	if err == nil {
		// Look for "HEAD branch: branch_name" in the output
		re := regexp.MustCompile(`(?m)^\s*HEAD branch:\s*(\S+)`)
		matches := re.FindStringSubmatch(string(out))
		if len(matches) > 1 {
			defaultBranch := strings.TrimSpace(matches[1])
			return branch == defaultBranch
		}
	}

	// Fallback to common default branch names if we can't determine from remote
	return branch == "main" || branch == "master"
}

// tagExists checks if a git tag exists
func tagExists(tagName string) bool {
	out, err := exec.Command("git", "tag", "-l", tagName).Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// pushCurrentBranch pushes the current branch to the remote
func pushCurrentBranch() error {
	// Get current branch name
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchOut, err := branchCmd.Output()
	if err != nil {
		return fmt.Errorf("could not get current branch: %v", err)
	}
	branchName := strings.TrimSpace(string(branchOut))

	// Push the branch with --set-upstream
	pushCmd := exec.Command("git", "push", "--set-upstream", "origin", branchName)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr

	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("error pushing branch: %v", err)
	}

	return nil
}

func pushTag(tagName string) error {
	// Check if remote exists
	out, err := exec.Command("git", "remote").Output()
	if err != nil || len(strings.TrimSpace(string(out))) == 0 {
		return fmt.Errorf("no remote repository configured. Please add a remote with 'git remote add origin <url>'")
	}

	// Try to push with dry-run first, but don't fail if it doesn't work
	testPush := exec.Command("git", "push", "--dry-run", "--no-verify", "origin", tagName)
	if err := testPush.Run(); err != nil {
		fmt.Printf("Dry-run push failed, will attempt actual push: %v\n", err)
	}

	// Push the tag to remote with retry logic
	maxRetries := 2
	for i := 0; i <= maxRetries; i++ {
		pushCmd := exec.Command("git", "push", "origin", tagName)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr

		err := pushCmd.Run()
		if err == nil {
			break
		}

		if i == maxRetries {
			return fmt.Errorf("failed to push tag after %d attempts: %v", maxRetries+1, err)
		}

		fmt.Printf("Push attempt %d failed, retrying...\n", i+1)
		time.Sleep(1 * time.Second)
	}

	// Try to push the current branch if it's tracking a remote branch
	branchCmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	branchOut, err := branchCmd.Output()
	if err == nil {
		branchName := strings.TrimSpace(string(branchOut))
		trackCmd := exec.Command("git", "push", "--set-upstream", "origin", branchName)
		trackCmd.Stdout = os.Stdout
		trackCmd.Stderr = os.Stderr
		// We don't fail if this command fails, as the tag push was successful
		trackCmd.Run()
	}

	return nil
}

// shouldSkipCI checks if the commit message contains [skip-ci] or similar patterns
func shouldSkipCI(commitMsg string) bool {
	// Check for common skip-ci patterns (case insensitive)
	skipPatterns := []string{
		"[skip-ci]",
		"[ci skip]",
		"skip-checks: true",
	}

	commitMsg = strings.ToLower(commitMsg)
	for _, pattern := range skipPatterns {
		if strings.Contains(commitMsg, pattern) {
			return true
		}
	}
	return false
}

func createGitTag(tagName string) error {
	// Create annotated tag with a message that includes [skip-ci]
	tagMessage := fmt.Sprintf("Release %s [skip-ci]", tagName)
	cmd := exec.Command("git", "tag", "-a", tagName, "-m", tagMessage)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Check if the error is specifically about the tag already existing
		if strings.Contains(err.Error(), "tag '"+tagName+"' already exists") {
			fmt.Printf("Warning: Tag %s already exists. Skipping tag creation.\n", tagName)
			return nil // Treat as a warning, not a fatal error
		}
		return fmt.Errorf("error creating tag: %v", err)
	}

	// Verify the tag was actually created (only if it wasn't a "tag exists" warning)
	if !tagExists(tagName) {
		return fmt.Errorf("failed to verify creation of tag %s", tagName)
	}

	return nil
}

// addAndCommitChangelog adds the changelog file to git and commits it.
func addAndCommitChangelog(changelogPath, tagName string) error {
	// git add CHANGELOG.md
	addCmd := exec.Command("git", "add", changelogPath)
	if err := addCmd.Run(); err != nil {
		return fmt.Errorf("error adding changelog to git: %v", err)
	}

	// git commit -m "chore(release): update changelog for vX.Y.Z [skip-ci]"
	commitMessage := fmt.Sprintf("chore(release): update changelog for %s [skip-ci]", tagName)
	commitCmd := exec.Command("git", "commit", "-m", commitMessage)
	commitCmd.Stdout = os.Stdout
	commitCmd.Stderr = os.Stderr
	if err := commitCmd.Run(); err != nil {
		return fmt.Errorf("error committing changelog: %v", err)
	}
	return nil
}

