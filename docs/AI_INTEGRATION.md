# AI-Powered Breaking Change Detection

Terranovate can use OpenAI's API to provide intelligent analysis of breaking changes in module and provider updates.

## Features

- 🤖 **Intelligent Analysis**: Uses GPT models to analyze version changes and detect potential breaking changes
- 📊 **Confidence Levels**: Provides confidence scores (high, medium, low) for each analysis
- 📝 **Detailed Insights**: Summarizes changes and provides specific details about what may break
- 🔄 **Automatic Integration**: Seamlessly integrates with the existing check workflow

## Configuration

### Option 1: Environment Variable (Recommended)

```bash
export OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxx
```

Then enable AI analysis in `.terranovate.yaml`:

```yaml
openai:
  enabled: true
  model: gpt-4o-mini  # Optional, this is the default
```

### Option 2: Configuration File

Add to your `.terranovate.yaml`:

```yaml
openai:
  enabled: true
  api_key: sk-xxxxxxxxxxxxxxxxxxxx
  model: gpt-4o-mini  # Optional
  base_url: https://api.openai.com/v1  # Optional, for custom endpoints
```

### Available Models

- `gpt-4o-mini` (default) - Fast and cost-effective
- `gpt-4o` - More capable, higher cost
- `gpt-4-turbo` - Balanced performance

## Usage

Once configured, AI analysis runs automatically with the `check` command:

```bash
terranovate check --path ./infrastructure
```

### Example Output

```
🔍 Found 1 update(s) available (1 with potential breaking changes ⚠️)
⚠️  1. vpc (major update)
   📍 Source: terraform-aws-modules/vpc/aws
   🔄 Current: 5.0.0 → Latest: 6.0.0
   ⚠️  BREAKING CHANGE: Major version upgrade from 5.0.0 to 6.0.0 may contain breaking changes.
   🤖 AI Analysis (high confidence):
      This is a major version update with significant API changes.
      • New required variable 'enable_vpc_endpoints' must be added
      • Variable 'enable_nat_gateway' has been removed
      • Output 'vpc_endpoint_ids' structure has changed
   📄 File: main.tf:10
   📋 Changelog: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.0.0
```

## How It Works

1. **Detection**: Terranovate detects available updates for modules and providers
2. **Analysis**: If AI is enabled, it sends version information and changelog URLs to OpenAI
3. **Evaluation**: The AI analyzes the changes using context about Terraform best practices
4. **Reporting**: Results are displayed with confidence levels and specific details

## Best Practices

### 1. Use Environment Variables for API Keys

Never commit API keys to version control:

```bash
# Good
export OPENAI_API_KEY=sk-xxxxxxxxxxxxxxxxxxxx

# Bad - Don't do this
openai:
  api_key: sk-xxxxxxxxxxxxxxxxxxxx  # Committed to git
```

### 2. Review AI Analysis

AI analysis is a helpful tool but should not replace human review:

- ✅ Use AI insights to identify potential issues
- ✅ Review changelogs and documentation
- ✅ Test changes in non-production environments
- ❌ Don't blindly trust AI analysis for critical infrastructure

### 3. Choose the Right Model

- **Development/Testing**: Use `gpt-4o-mini` for cost-effective analysis
- **Production**: Consider `gpt-4o` for higher accuracy on critical updates

### 4. Monitor Costs

OpenAI API usage incurs costs. Each analysis makes one API call:

- `gpt-4o-mini`: ~$0.0001-0.0002 per analysis
- `gpt-4o`: ~$0.001-0.002 per analysis

For 100 module updates per month:
- `gpt-4o-mini`: ~$0.01-0.02/month
- `gpt-4o`: ~$0.10-0.20/month

## Troubleshooting

### No AI Analysis Appearing

Check the logs with verbose mode:

```bash
terranovate check --path ./infra -v
```

Look for messages like:
- `AI-powered breaking change detection enabled` - AI is configured
- `AI analysis enabled but no API key configured` - API key missing

### API Errors

Common issues:

1. **Invalid API Key**
   ```
   OpenAI API error: Incorrect API key provided
   ```
   Solution: Verify your `OPENAI_API_KEY` is correct

2. **Rate Limiting**
   ```
   OpenAI API returned status 429
   ```
   Solution: Reduce frequency or upgrade your OpenAI plan

3. **Timeout**
   ```
   failed to call OpenAI API: context deadline exceeded
   ```
   Solution: Check network connectivity or increase timeout

## Azure OpenAI Support

Terranovate supports Azure OpenAI Service:

```yaml
openai:
  enabled: true
  api_key: your-azure-key
  model: gpt-4o-mini
  base_url: https://your-resource.openai.azure.com/openai/deployments/your-deployment
```

## Privacy and Security

### Data Sent to OpenAI

When AI analysis is enabled, Terranovate sends:
- Module/provider name
- Current version
- Latest version
- Changelog URL

**NOT sent:**
- Your infrastructure code
- Secrets or credentials
- Internal documentation
- Private repository contents

### Disabling AI Analysis

To disable AI analysis:

```yaml
openai:
  enabled: false
```

Or simply don't set the `OPENAI_API_KEY` environment variable.

## CI/CD Integration

### GitHub Actions

```yaml
- name: Check for updates with AI
  env:
    GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
    OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
  run: |
    terranovate check --path ./infrastructure
```

### GitLab CI

```yaml
variables:
  OPENAI_API_KEY: ${OPENAI_API_KEY}

terranovate:
  script:
    - terranovate check --path ./infrastructure
```

## Limitations

1. **Internet Access Required**: AI analysis requires connectivity to OpenAI API
2. **Cost**: API usage incurs costs (though minimal with gpt-4o-mini)
3. **Rate Limits**: Subject to OpenAI API rate limits
4. **Accuracy**: AI analysis is probabilistic and may not catch all breaking changes
5. **Language**: Best results with English changelogs and documentation

## FAQ

**Q: Is AI analysis required?**
A: No, it's completely optional. Terranovate works perfectly without AI.

**Q: Will it slow down my checks?**
A: Yes, slightly. Each module update adds ~1-3 seconds for AI analysis. Analyses run sequentially.

**Q: Can I use it offline?**
A: No, AI analysis requires internet access to OpenAI API.

**Q: Does it work with private modules?**
A: Yes, but the AI only sees version numbers and public changelog URLs.

**Q: What if OpenAI is down?**
A: Terranovate will log a warning and continue without AI analysis.

## Support

For issues or questions about AI integration:
- Open an issue: https://github.com/heyjobs/terranovate/issues
- Check logs with `-v` flag for debugging
