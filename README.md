# SemVerGo

**SemVerGo** is a powerful and flexible open-source CLI tool written in Go. It automates semantic versioning and changelog generation for Git repositories that adhere to the [Conventional Commits](https://www.conventionalcommits.org/) standard.

More than just a versioning utility, SemVerGo streamlines your release workflows by analyzing commit history, determining the next semantic version, tagging the release, and optionally generating and committing a changelog. It supports CI/CD pipelines, pre-release versions, dry run previews, and customizable tag formats — making your release process consistent, traceable, and effortless.

---

## 🚀 Features

- **Semantic Versioning**  
  Analyzes the Git commit history based on the Conventional Commits format to determine the next appropriate semantic version (MAJOR.MINOR.PATCH). Automatically creates a Git tag unless instructed otherwise.

- **Conventional Commits Support**  
  Parses commit messages structured with Conventional Commits and maps them to semantic version changes (e.g., `feat:` = MINOR, `fix:` = PATCH, `BREAKING CHANGE:` = MAJOR).

- **Automatic Changelog Generation**  
  Builds a human-readable `CHANGELOG.md` using parsed commit messages, keeping your project history well-documented and standardized.

- **Pre-release Versioning**  
  Generates pre-release tags (e.g., `-beta.1`, `-rc.0`) automatically based on non-main branch names, helping with release candidates and development versions.

- **Dry Run Mode**  
  Preview all actions (version bump, changelog output, Git tagging) without making any changes — perfect for testing or validation in CI environments.

- **CI/CD Compatibility**  
  Designed with automation in mind. Auto-detects branch names, pushes Git tags, and supports headless environments with `-ci` mode.

- **Changelog Commit Automation**  
  Automatically stages and commits the `CHANGELOG.md` file when generated, reducing manual steps and human error.

- **Custom Tag Format**  
  Supports customizable Git tag naming using templates such as `v{{.Major}}.{{.Minor}}.{{.Patch}}{{.Prerelease}}` or `release-{{.Major}}.{{.Minor}}.{{.Patch}}`.

---

## 🛠 Installation

### Via `go install`

```bash
go install github.com/emrefirat/SemVerGo@latest
```

### From Source

```bash
git clone https://github.com/emrefirat/SemVerGo.git
cd SemVerGo
go build -o semvergo .
```

> You may want to move `semvergo` to a directory in your system's `PATH` to run it globally.

---

## 📦 Usage

### Basic Usage

```bash
./semvergo
```

This command runs in the current Git repository, determines the latest tag, analyzes commit history using the Conventional Commits format, calculates the next semantic version, and creates a Git tag.

---

### Command-line Options

| Flag                | Description |
|---------------------|-------------|
| `-branch string`    | Branch name (default: current branch). |
| `-ci`               | Run in CI mode: automatically detects the branch, generates changelog, and pushes tags. |
| `-debug`            | Enable verbose debug output for detailed logs. |
| `-dry-run`          | Preview actions (version bump, changelog generation, Git operations) without making changes. |
| `-git-address string` | Path to the Git repository SemVerGo should operate on (default: current directory). |
| `-next-version-only` | Outputs only the next calculated version and exits. Does not tag or generate changelog. |
| `-output-changelog` | Enables generation and auto-commit of `CHANGELOG.md` using Conventional Commits. |
| `-preRelease`       | Enables pre-release versioning based on the current branch (e.g., `v1.2.3-feature.branch.0`). Enabled automatically for non-main branches. |
| `-push-branch`      | Pushes the local branch to the remote repository if it doesn't exist or is behind. |
| `-skip-checks`      | Skips Git configuration and working directory status checks (use with caution). |
| `-tag-format string` | Custom format for Git tags. Placeholders: `{{.Major}}`, `{{.Minor}}`, `{{.Patch}}`, `{{.Prerelease}}`. Example: `release-{{.Major}}.{{.Minor}}.{{.Patch}}`. |
| `-version string`   | Manually specify a version (e.g., `1.2.3`). If provided, SemVerGo will not analyze commits. |

---

## 🧪 Examples

### 🔍 Dry Run (Preview)

```bash
./semvergo -dry-run -output-changelog
```

> Shows what version would be generated and what changelog content would be created, without modifying your repo.

---

### 🚀 Full Release with Changelog

```bash
./semvergo -output-changelog
```

> Calculates next version, generates `CHANGELOG.md`, commits it, and creates a Git tag.

---

### 🧪 CI/CD Release

```bash
./semvergo -ci -output-changelog
```

> Ideal for automated pipelines: auto-detects branch, outputs changelog, creates tag, and pushes changes.

---

### 🔖 Pre-release Versioning

```bash
./semvergo -preRelease -output-changelog
```

> Generates a pre-release version such as `v1.4.0-feature.cool.0` for non-main branches.

---

## 📝 Changelog Management

When `-output-changelog` is enabled, SemVerGo:

- Creates or updates a `CHANGELOG.md` file using Conventional Commits.
- Automatically stages (`git add`) and commits it with a descriptive message.
- If used in CI mode, it can also push the changelog commit.

> No manual changelog editing or commits required — it's all automatic!

