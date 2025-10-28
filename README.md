# Terranovate

**Automated Terraform module and provider update tool**

Terranovate automatically detects outdated Terraform modules and providers (from Terraform Registry and Git sources), validates updates via `terraform plan`, and opens automated Pull Requests in GitHub.

[![Test](https://github.com/heyjobs/terranovate/workflows/Test/badge.svg)](https://github.com/heyjobs/terranovate/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/heyjobs/terranovate)](https://goreportcard.com/report/github.com/heyjobs/terranovate)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Features

- 🔍 **Automatic Module Detection**: Scans Terraform files for module usage
- 🔌 **Provider Version Checking**: Automatically detects and updates Terraform providers
- 🧹 **Unused Provider Detection**: Identifies providers declared but not actually used
- 📦 **Multi-Source Support**: Works with Terraform Registry and Git-based modules
- ⚠️ **Three-Layer Breaking Change Detection**:
  - Semantic version analysis (major/minor/patch)
  - Infrastructure impact (resource replacements, deletions)
  - API/schema comparison (variable and output changes)
- 🔬 **Resource Change Analysis**: Shows exactly which resources will be replaced, deleted, or modified
- 📋 **API Change Detection**: Identifies new required variables, removed variables, and type changes
- ✅ **Validation**: Runs `terraform plan` before creating PRs
- 🤖 **GitHub Integration**: Automatically creates Pull Requests with detailed changelogs
- 💬 **PR Checks**: Comments on pull requests with dependency status and breaking changes
- 🏷️ **Smart Labeling**: Automatically labels PRs by update type and breaking changes
- 🔔 **Notifications**: Slack notifications and JSON output for CI integration
- 📝 **Multiple Output Formats**: Text, JSON, and GitHub-flavored Markdown
- ⚙️ **Configurable**: Flexible YAML configuration for different workflows
- 🚀 **Production Ready**: Comprehensive tests, CI/CD, and Docker support

## Quick Start

### Installation

#### Using Go

```bash
go install github.com/heyjobs/terranovate@latest
```

#### Using Make

```bash
git clone https://github.com/heyjobs/terranovate.git
cd terranovate
make install
```

#### Using Docker

```bash
docker pull ghcr.io/heyjobs/terranovate:latest
```

### Basic Usage

```bash
# Scan for Terraform modules
terranovate scan --path ./infrastructure

# Check for available updates
terranovate check --path ./infrastructure

# Check with markdown output (for PR comments)
terranovate check --path ./infrastructure --format markdown

# Run terraform plan
terranovate plan --path ./infrastructure

# Create pull requests for updates
export GITHUB_TOKEN=ghp_xxxxx
terranovate pr --repo owner/repo --path ./infrastructure

# Send notifications
terranovate notify --format slack
```

## Commands

### `check`

Compares current module and provider versions with the latest available versions.

```bash
terranovate check --path ./infra

# Output as markdown for PR comments
terranovate check --path ./infra --format markdown
```

**Options:**
- `--path, -p`: Path to scan for Terraform files
- `--format, -f`: Output format: `text` (default) or `markdown` (for PR comments)
- `--check-unused-providers`: Check for unused providers (default: true)

**Example Output (text format):**
```
Found 2 update(s) available (1 with potential breaking changes)

⚠️  1. vpc (major update)
   Source: terraform-aws-modules/vpc/aws
   Current: 5.0.0 → Latest: 6.0.0
   ⚠️  BREAKING CHANGE: Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes. Please review the changelog carefully.
   File: main.tf:10
   Changelog: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.0.0

📦 2. eks (minor update)
   Source: git::https://github.com/terraform-aws-modules/terraform-aws-eks.git
   Current: v19.0.0 → Latest: 19.16.0
   File: eks.tf:5
   Changelog: https://github.com/terraform-aws-modules/terraform-aws-eks/releases/tag/v19.16.0

============================================================
Provider Updates
============================================================

Found 1 provider update(s) available

⚠️  1. aws (major update)
   Source: hashicorp/aws
   Current: 5.0.0 → Latest: 6.0.0
   ⚠️  BREAKING CHANGE: Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes.
   File: providers.tf:5
   Documentation: https://registry.terraform.io/providers/hashicorp/aws/6.0.0
```

The **markdown format** is specially designed for GitHub PR comments with:
- Collapsible `<details>` sections for each update
- Tables showing version and source information
- Clear visual indicators for breaking changes
- Direct links to changelogs and documentation

### `scan`

Scans Terraform files and extracts all module blocks with their sources and versions.

```bash
terranovate scan --path ./infra
```

**Options:**
- `--path, -p`: Path to scan for Terraform files (default: current directory)

**Example Output:**
```
Found 3 module(s):

1. vpc
   Source: terraform-aws-modules/vpc/aws
   Version: 5.0.0
   Type: registry
   File: main.tf:10

2. eks
   Source: git::https://github.com/terraform-aws-modules/terraform-aws-eks.git?ref=v19.0.0
   Version:
   Type: git
   File: eks.tf:5
```

### `check`

Compares current module versions with the latest available versions.

```bash
terranovate check --path ./infra
```

**Options:**
- `--path, -p`: Path to scan for Terraform files

**Example Output:**
```
Found 2 update(s) available (1 with potential breaking changes)

⚠️  1. vpc (major update)
   Source: terraform-aws-modules/vpc/aws
   Current: 5.0.0 → Latest: 6.0.0
   ⚠️  BREAKING CHANGE: Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes. Please review the changelog carefully.
   File: main.tf:10
   Changelog: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.0.0

📦 2. eks (minor update)
   Source: git::https://github.com/terraform-aws-modules/terraform-aws-eks.git
   Current: v19.0.0 → Latest: 19.16.0
   File: eks.tf:5
   Changelog: https://github.com/terraform-aws-modules/terraform-aws-eks/releases/tag/v19.16.0

⚠️  Warning: 1 update(s) may contain breaking changes.
   Please review changelogs carefully before applying these updates.
```

### `plan`

Runs `terraform init` and `terraform plan` to validate infrastructure changes.

```bash
terranovate plan --path ./infra
```

**Options:**
- `--path, -p`: Path to Terraform working directory

**Example Output:**
```
Running terraform init...
✓ Terraform init completed

Running terraform plan...
✓ Terraform plan completed successfully

Plan: 2 to add, 1 to change, 0 to destroy

⚠️  Infrastructure changes detected:
   + 2 resource(s) to add
   ~ 1 resource(s) to change
```

### `pr`

Creates GitHub Pull Requests for module updates.

```bash
terranovate pr --repo heyjobs/platform-infra --path ./infra
```

**Options:**
- `--repo, -r`: GitHub repository (format: owner/repo)
- `--owner`: GitHub repository owner
- `--path, -p`: Path to Terraform working directory
- `--skip-plan`: Skip terraform plan validation

**Example Output:**
```
Found 2 update(s) available

[1/2] Processing vpc...
  ✓ PR created: https://github.com/heyjobs/platform-infra/pull/123
  #123: Update Terraform module vpc to 5.1.2

[2/2] Processing eks...
  ✓ PR created: https://github.com/heyjobs/platform-infra/pull/124
  #124: Update Terraform module eks to 19.16.0

✓ Successfully created 2/2 pull request(s)
```

### `notify`

Sends notifications about available updates.

```bash
# Slack notification
terranovate notify --format slack

# JSON output
terranovate notify --format json

# Text output
terranovate notify --format text
```

**Options:**
- `--path, -p`: Path to scan for Terraform files
- `--format, -f`: Output format (slack, json, text)

## GitHub Token Setup

Terranovate requires a GitHub token to:
- ✅ **Avoid Rate Limiting**: 5,000 requests/hour (vs 60 without token)
- ✅ **Access Private Repositories**: Query private module sources
- ✅ **Create Pull Requests**: Automatically open PRs for updates

### Creating a Token

1. Go to [GitHub Settings → Developer settings → Personal access tokens](https://github.com/settings/tokens)
2. Click "Generate new token (classic)"
3. Select scopes:
   - `repo` - Full control of private repositories (required for private repos)
   - `public_repo` - Access to public repositories only (if only using public repos)
4. Generate and copy the token

### Using the Token

**Option 1: Environment Variable (Recommended)**
```bash
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
terranovate check --path ./infrastructure
```

**Option 2: Configuration File**
```yaml
# .terranovate.yaml
github:
  token: ghp_xxxxxxxxxxxxxxxxxxxx
```

**Option 3: CI/CD Secrets**
```yaml
# GitHub Actions
env:
  GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

# GitLab CI
variables:
  GITHUB_TOKEN: ${GITHUB_TOKEN}
```

### Caching

Terranovate automatically caches GitHub repository data in memory to minimize API calls:

- **Cache Type**: In-memory only (no disk persistence)
- **Cache Duration**: 24 hours (or until process ends)
- **Cache Scope**: Repository tags and version information
- **Benefits**:
  - Reduces API calls by 90%+ within the same session
  - Fast access with no disk I/O
  - Automatic cleanup when process exits
  - Privacy-friendly (no sensitive data written to disk)

The cache is automatically managed - no configuration needed!

## Configuration

Create a `.terranovate.yaml` file in your repository:

```yaml
# Terraform configuration
terraform:
  working_dir: ./infrastructure
  env:
    AWS_REGION: us-east-1

# GitHub configuration
github:
  # IMPORTANT: Token is highly recommended to avoid rate limiting and access private repos
  token: ghp_xxxxxxxxxxxxxxxxxxxx  # or use GITHUB_TOKEN env var
  owner: heyjobs
  repo: platform-infra
  base_branch: main
  labels:
    - terraform
    - dependencies
  reviewers:
    - platform-team

# Scanner configuration
scanner:
  include:
    - "*.tf"
  exclude:
    - ".terraform"
    - "examples"
  recursive: true

# Version checking
version_check:
  skip_prerelease: true
  patch_only: false
  minor_only: false
  ignore_modules:
    - legacy-vpc
  ignore_unused_providers:
    - null
    - random
    - time

# Notifications
notifier:
  output_format: text
  slack:
    enabled: true
    webhook_url: https://hooks.slack.com/services/XXX/YYY/ZZZ
    channel: "#terraform-updates"
```

## CI/CD Integration

### GitHub Actions - Automated PR Creation

Run Terranovate on a schedule to automatically create PRs for module updates:

```yaml
name: Terranovate

on:
  schedule:
    - cron: '0 9 * * MON'  # Run every Monday at 9 AM
  workflow_dispatch:

jobs:
  update-modules:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Run Terranovate
        uses: docker://ghcr.io/heyjobs/terranovate:latest
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          args: pr --repo ${{ github.repository }} --path ./infrastructure
```

### GitHub Actions - PR Dependency Checks

Add automated dependency checks to your pull requests. Terranovate will comment on PRs with information about outdated dependencies in the changed files.

#### Option 1: Check Only Changed Directories

This workflow only checks directories that contain changed Terraform files, making it fast and focused:

```yaml
name: Terraform Dependency Check

on:
  pull_request:
    paths:
      - '**.tf'
      - '**.tfvars'

permissions:
  contents: read
  pull-requests: write

jobs:
  terraform-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Get changed Terraform directories
        id: changed-dirs
        run: |
          CHANGED_FILES=$(git diff --name-only origin/${{ github.base_ref }}...HEAD | grep '\.tf$' || true)
          if [ -z "$CHANGED_FILES" ]; then
            echo "directories=" >> $GITHUB_OUTPUT
            exit 0
          fi
          DIRECTORIES=$(echo "$CHANGED_FILES" | xargs -n1 dirname | sort -u | tr '\n' ' ')
          echo "directories=$DIRECTORIES" >> $GITHUB_OUTPUT

      - uses: actions/setup-go@v5
        if: steps.changed-dirs.outputs.directories != ''
        with:
          go-version: '1.23'

      - name: Install Terranovate
        if: steps.changed-dirs.outputs.directories != ''
        run: go install github.com/heyjobs/terranovate@latest

      - name: Run Terranovate checks
        if: steps.changed-dirs.outputs.directories != ''
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          OUTPUT_FILE="terranovate-output.md"
          echo "## 🔍 Terranovate Dependency Check" > $OUTPUT_FILE

          for DIR in ${{ steps.changed-dirs.outputs.directories }}; do
            echo "### 📁 Directory: \`$DIR\`" >> $OUTPUT_FILE
            terranovate check --path "$DIR" --format markdown >> $OUTPUT_FILE
          done

      - name: Comment PR
        if: steps.changed-dirs.outputs.directories != ''
        uses: actions/github-script@v7
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          script: |
            const fs = require('fs');
            const body = fs.readFileSync('terranovate-output.md', 'utf8');

            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });

            const botComment = comments.find(comment =>
              comment.user.type === 'Bot' &&
              comment.body.includes('Terranovate Dependency Check')
            );

            if (botComment) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body: body
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body: body
              });
            }
```

#### Option 2: Check All Directories

This workflow checks all Terraform directories in your repository on every PR, providing comprehensive visibility:

```yaml
name: Terraform Dependency Check (All)

on:
  pull_request:
    types: [opened, synchronize, reopened]

permissions:
  contents: read
  pull-requests: write

jobs:
  terraform-check-all:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Install Terranovate
        run: go install github.com/heyjobs/terranovate@latest

      - name: Find and check all Terraform directories
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          OUTPUT_FILE="terranovate-output.md"
          echo "## 🔍 Terranovate Dependency Check" > $OUTPUT_FILE

          find . -name "*.tf" -type f -exec dirname {} \; | sort -u | grep -v ".terraform" | while read DIR; do
            TEMP_OUTPUT=$(mktemp)
            if terranovate check --path "$DIR" --format markdown > $TEMP_OUTPUT 2>&1; then
              if grep -q "update(s) available" $TEMP_OUTPUT; then
                echo "### 📁 Directory: \`$DIR\`" >> $OUTPUT_FILE
                cat $TEMP_OUTPUT >> $OUTPUT_FILE
              fi
            fi
            rm -f $TEMP_OUTPUT
          done

      - name: Comment PR
        uses: actions/github-script@v7
        with:
          github-token: ${{ secrets.GITHUB_TOKEN }}
          script: |
            const fs = require('fs');
            let body = fs.readFileSync('terranovate-output.md', 'utf8');

            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });

            const botComment = comments.find(comment =>
              comment.user.type === 'Bot' &&
              comment.body.includes('Terranovate Dependency Check')
            );

            if (botComment) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: botComment.id,
                body: body
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body: body
              });
            }
```

**Features:**
- 📝 Automatically comments on PRs with dependency status
- 🔄 Updates existing comments instead of creating new ones
- ⚠️ Highlights breaking changes with warnings
- 📦 Shows both module and provider updates
- 🎨 Clean, collapsible markdown format
- 🏷️ Optional: Auto-label PRs with `dependencies` and `breaking-change` labels

**Full workflow files** are available in [`.github/workflows/`](.github/workflows/):
- [`pr-terraform-check.yml`](.github/workflows/pr-terraform-check.yml) - Check changed directories only
- [`pr-terraform-check-all.yml`](.github/workflows/pr-terraform-check-all.yml) - Check all directories

### GitLab CI

```yaml
terranovate:
  image: ghcr.io/heyjobs/terranovate:latest
  script:
    - terranovate pr --repo ${CI_PROJECT_PATH} --path ./infrastructure
  only:
    - schedules
  variables:
    GITHUB_TOKEN: ${GITHUB_TOKEN}
```

## Advanced Breaking Change Detection

Terranovate provides **three levels of breaking change detection** to give you complete visibility into module updates:

### 1. Semantic Version Analysis

Analyzes version numbers according to [SemVer 2.0.0](https://semver.org/):

- **Major Version** (X.0.0 → X+1.0.0): Breaking changes expected
- **Minor Version** (X.Y.0 → X.Y+1.0): New features, backwards compatible
- **Patch Version** (X.Y.Z → X.Y.Z+1): Bug fixes only

### 2. Infrastructure Impact Analysis

Parses `terraform plan` output to detect:

- **Resource Replacements**: Resources that will be destroyed and recreated
- **Resource Deletions**: Resources that will be removed
- **In-place Modifications**: Attributes updated without replacement

**Example Output:**
```bash
Resource Changes:
⚠️  2 resource(s) will be REPLACED:
   - aws_instance.web (One or more immutable attributes changed)
   - aws_db_instance.main (One or more immutable attributes changed)
🗑️  1 resource(s) will be DELETED:
   - aws_s3_bucket.deprecated
📝 3 resource(s) will be MODIFIED
```

### 3. API/Schema Comparison

Compares module interfaces between versions:

- **Added Required Variables**: New mandatory inputs
- **Removed Variables**: Inputs no longer accepted
- **Changed Variable Types**: Type modifications (string → list, etc.)
- **Removed Outputs**: Outputs no longer available

**Example Output:**
```bash
API/Schema Changes Detected:

⚠️ New Required Variables (2)
- enable_vpc_endpoints (bool)
  - Enable VPC endpoints for private AWS services
- availability_zones (list(string))
  - List of AZs to deploy resources

🗑️ Removed Variables (1)
- enable_nat_gateway (bool)

⚙️ Changed Variable Types (1)
- subnet_ids: string → list(string)
```

### Combined Detection Example

```bash
$ terranovate check --path ./infrastructure

Found 1 update(s) available (1 with potential breaking changes)

⚠️  1. vpc (major update)
   Source: terraform-aws-modules/vpc/aws
   Current: 5.0.0 → Latest: 6.0.0
   ⚠️  BREAKING CHANGE: Major version upgrade + API changes + resource replacements

   API/Schema Changes:
   ⚠️  2 new required variable(s) must be added
   🗑️  1 variable removed

   Resource Changes:
   ⚠️  3 resource(s) will be REPLACED
   🗑️  1 resource(s) will be DELETED

   File: main.tf:10
   Changelog: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.0.0
```

### Pull Request Integration

PRs automatically include:

1. **Warning Banner** for breaking changes
2. **API/Schema Changes Section** with detailed variable/output changes
3. **Resource Changes Section** with exact resources affected
4. **Review Checklist** customized for the type of changes
5. **Terraform Plan Output** for validation
6. **Automatic Labels**: `breaking-change`, `major-update`, etc.

### Best Practices

1. ✅ **Review all three detection layers** - version, API, and resources
2. ✅ **Check for new required variables** - update your module calls
3. ✅ **Verify resource replacements** - ensure downtime is acceptable
4. ✅ **Test in non-production first** - validate in staging/dev
5. ✅ **Coordinate team** - breaking changes need team awareness
6. ✅ **Update documentation** - reflect API changes in your docs

## Provider Version Checking

In addition to checking module versions, Terranovate also automatically scans and checks for updates to your Terraform provider versions.

### How It Works

Terranovate scans your Terraform files for `terraform.required_providers` blocks and checks the Terraform Registry for the latest available versions:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"  # Terranovate will check for latest 5.x version
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 4.0.0"  # Terranovate will check for latest version
    }
  }
}
```

### Provider Update Detection

```bash
$ terranovate check --path ./infrastructure

Found 2 update(s) available

📦 1. vpc (minor update)
   Source: terraform-aws-modules/vpc/aws
   Current: 5.0.0 → Latest: 5.1.0
   File: main.tf:10

============================================================
Provider Updates
============================================================

Found 2 provider update(s) available (1 with potential breaking changes)

⚠️  1. aws (major update)
   Source: hashicorp/aws
   Current: 5.0.0 → Latest: 6.0.0
   ⚠️  BREAKING CHANGE: Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes.
   File: providers.tf:5
   Documentation: https://registry.terraform.io/providers/hashicorp/aws/6.0.0

📦 2. google (minor update)
   Source: hashicorp/google
   Current: 4.50.0 → Latest: 4.80.0
   File: providers.tf:10
   Documentation: https://registry.terraform.io/providers/hashicorp/google/4.80.0
```

### Provider Pull Requests

When creating PRs, Terranovate automatically handles both module AND provider updates:

```bash
$ terranovate pr --repo owner/repo --path ./infrastructure

Found 3 update(s) available (2 modules, 1 providers)

[1/3] Processing module vpc...
  ✓ PR created: https://github.com/owner/repo/pull/123

[2/3] Processing module eks...
  ✓ PR created: https://github.com/owner/repo/pull/124

[3/3] Processing provider aws...
  ✓ PR created: https://github.com/owner/repo/pull/125

✓ Successfully created 3/3 pull request(s)
```

### Provider PR Features

Provider PRs include:

- 🏷️ **Automatic Labels**: `provider`, `breaking-change`, `major-update`, etc.
- ⚠️ **Breaking Change Detection**: Major version updates are flagged
- 📝 **Custom Review Checklist**: Provider-specific checklist items
- 🔗 **Documentation Links**: Direct links to provider documentation
- 🔄 **Version Constraint Preservation**: Maintains your constraint operators (`~>`, `>=`, etc.)

**Example Provider PR Body:**

```markdown
## ⚠️ Breaking Change Warning

Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes. Please review the changelog carefully.

**Please review the provider documentation and upgrade guide carefully before merging.**

---

## Provider Update

Updates the **aws** provider.

### Update Details

- **Provider**: `hashicorp/aws`
- **Current Version**: `5.0.0`
- **New Version**: `6.0.0`
- **Update Type**: 🔴 Major
- **File**: `providers.tf:5`

📖 [View provider documentation](https://registry.terraform.io/providers/hashicorp/aws/6.0.0)

### Review Checklist

- [ ] Review provider upgrade guide and breaking changes
- [ ] Check for deprecated resources or data sources
- [ ] Verify all resource configurations are compatible
- [ ] Review provider changelog
- [ ] Run `terraform init -upgrade` to update provider
- [ ] Run `terraform plan` to verify no unexpected changes
- [ ] Test in non-production environment first
- [ ] Update resource configurations if needed
- [ ] Communicate changes to team
```

### Supported Provider Sources

Terranovate works with all providers from the Terraform Registry:

```hcl
terraform {
  required_providers {
    # HashiCorp providers
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }

    # Community providers
    datadog = {
      source  = "datadog/datadog"
      version = ">= 3.0"
    }

    # Partner providers
    mongodbatlas = {
      source  = "mongodb/mongodbatlas"
      version = "1.10.0"
    }
  }
}
```

### Provider Update Best Practices

1. ✅ **Test provider updates in non-production first**
2. ✅ **Review provider upgrade guides** - especially for major versions
3. ✅ **Check for deprecated resources** - update before they're removed
4. ✅ **Run `terraform plan`** - verify no unexpected changes
5. ✅ **Update one provider at a time** - easier to identify issues
6. ✅ **Monitor provider changelogs** - stay informed about upcoming changes

## Unused Provider Detection

Terranovate can automatically detect providers that are declared in your `required_providers` block but not actually used by any resources or data sources in your code.

### How It Works

When you run `terranovate check --check-unused-providers`, it:

1. Scans your Terraform files for `required_providers` declarations
2. Scans for all `resource` and `data` blocks
3. Extracts provider names from resource types (e.g., `aws_instance` → `aws`)
4. Compares declared providers against actually used providers
5. Reports any providers that are declared but not used

### Usage

```bash
# Check for unused providers (enabled by default)
terranovate check --path ./infrastructure

# Disable unused provider check
terranovate check --path ./infrastructure --check-unused-providers=false
```

### Example Output

```bash
$ terranovate check --path ./infrastructure

============================================================
🔌 Unused Providers
============================================================

🔍 Found 2 unused provider(s)

⚠️  1. google
   📍 Source: hashicorp/google
   🔖 Version: >= 4.0.0
   📄 File: providers.tf:8
   💡 Suggestion: Consider removing this provider if it's not needed, or check if resources are defined in child modules.

⚠️  2. azurerm
   📍 Source: hashicorp/azurerm
   🔖 Version: 3.50.0
   📄 File: providers.tf:12
   💡 Suggestion: Consider removing this provider if it's not needed, or check if resources are defined in child modules.

ℹ️  These providers are declared in required_providers but not used by any resources.
   Consider removing them to keep your configuration clean.
   Use --check-unused-providers=false to disable this check.
```

### Configuration

You can configure providers to ignore in your `.terranovate.yaml` file:

```yaml
version_check:
  # Ignore these providers when checking for unused providers
  ignore_unused_providers:
    - null      # Utility provider, may be used implicitly
    - random    # Utility provider, may be used implicitly
    - time      # Utility provider, may be used implicitly
    - local     # May be used by modules only
```

### Use Cases

**Clean up unused providers:**
```hcl
# Before
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    google = {  # Not used anywhere!
      source  = "hashicorp/google"
      version = ">= 4.0.0"
    }
  }
}

resource "aws_instance" "web" {
  ami           = "ami-0c55b159cbfafe1f0"
  instance_type = "t2.micro"
}

# After (google provider removed)
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
```

**Utility Providers:**

Some providers like `null`, `random`, `time`, `external`, `local`, `archive`, `http`, `template`, and `tls` are utility providers that might be used:
- By child modules
- For implicit operations
- In conditional resources

Terranovate provides special suggestions for these providers and recommends verifying their usage before removal.

### Benefits

- 🧹 **Cleaner Configuration**: Remove unnecessary provider declarations
- 📉 **Faster Init**: Fewer providers to download during `terraform init`
- 🔒 **Better Security**: Reduce attack surface by removing unused dependencies
- 📊 **Clear Dependencies**: Make it obvious which cloud providers you're actually using

## Docker Usage

```bash
# Run scan
docker run --rm -v $(pwd):/workspace ghcr.io/heyjobs/terranovate:latest scan --path /workspace

# Run check
docker run --rm -v $(pwd):/workspace ghcr.io/heyjobs/terranovate:latest check --path /workspace

# Create PRs
docker run --rm -v $(pwd):/workspace \
  -e GITHUB_TOKEN=${GITHUB_TOKEN} \
  ghcr.io/heyjobs/terranovate:latest \
  pr --repo owner/repo --path /workspace
```

## Development

### Prerequisites

- Go 1.23+
- Terraform 1.0+
- Make
- Docker (optional)

### Building

```bash
# Clone repository
git clone https://github.com/heyjobs/terranovate.git
cd terranovate

# Download dependencies
make deps

# Build
make build

# Run tests
make test

# Run linter
make lint

# Build Docker image
make docker-build
```

### Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-coverage

# Run specific test
go test -v ./tests -run TestScanner_Scan
```

### Project Structure

```
terranovate/
├── cmd/                    # CLI commands
│   ├── root.go
│   ├── scan.go
│   ├── check.go
│   ├── plan.go
│   ├── pr.go
│   └── notify.go
├── internal/               # Internal packages
│   ├── scanner/           # Terraform file scanner
│   ├── version/           # Version checker
│   ├── terraform/         # Terraform runner
│   ├── github/            # GitHub PR creator
│   └── notifier/          # Notification handler
├── pkg/                   # Public packages
│   └── config/           # Configuration handling
├── tests/                # Test files
├── .github/workflows/    # CI configuration
├── Dockerfile           # Container image
├── Makefile            # Build automation
└── README.md           # This file
```

## How It Works

1. **Scan**: Terranovate uses HashiCorp's HCL parser to extract module blocks from `.tf` files
2. **Check**: Queries Terraform Registry API for registry modules, or GitHub API for git-based modules
3. **Validate**: Optionally runs `terraform plan` to ensure updates won't break infrastructure
4. **Create PR**: Uses GitHub API to create a new branch, commit changes, and open a Pull Request
5. **Notify**: Sends notifications via Slack or outputs JSON for further processing

## Supported Module Sources

### Terraform Registry

```hcl
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.0.0"
}
```

### Git (HTTPS)

```hcl
module "eks" {
  source = "git::https://github.com/terraform-aws-modules/terraform-aws-eks.git?ref=v19.0.0"
}
```

### Git (SSH)

```hcl
module "rds" {
  source = "git@github.com:terraform-aws-modules/terraform-aws-rds.git?ref=v5.0.0"
}
```

### GitHub (shorthand)

```hcl
module "s3" {
  source = "github.com/terraform-aws-modules/terraform-aws-s3-bucket?ref=v3.0.0"
}
```

## Limitations

- Local modules (e.g., `./modules/local`) are not checked for updates
- Private registries require additional authentication setup
- Git modules must use tags for version detection

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [HashiCorp](https://www.hashicorp.com/) for Terraform and HCL parser
- [Renovate](https://github.com/renovatebot/renovate) for inspiration
- [Dependabot](https://github.com/dependabot) for the dependency update workflow pattern

## Support

For issues, questions, or contributions, please use the [GitHub issue tracker](https://github.com/heyjobs/terranovate/issues).
