# AISH Integration Guide

This guide provides comprehensive instructions for integrating AISH with various systems, tools, and workflows.

## Table of Contents

- [Editor Integrations](#editor-integrations)
- [CI/CD Integration](#cicd-integration)
- [Container Integration](#container-integration)
- [Cloud Platform Integration](#cloud-platform-integration)
- [Custom Tool Integration](#custom-tool-integration)
- [Webhook Integration](#webhook-integration)

## Editor Integrations

### VS Code Extension

Create a VS Code extension that integrates AISH for terminal error analysis:

```json
// package.json
{
  "name": "aish-vscode",
  "displayName": "AISH Terminal Assistant",
  "description": "Intelligent terminal error analysis with AI",
  "version": "1.0.0",
  "engines": {
    "vscode": "^1.60.0"
  },
  "categories": ["Other"],
  "activationEvents": [
    "onCommand:aish.analyzeTerminal",
    "onTerminal:*"
  ],
  "main": "./out/extension.js",
  "contributes": {
    "commands": [
      {
        "command": "aish.analyzeTerminal",
        "title": "Analyze Terminal Error",
        "category": "AISH"
      },
      {
        "command": "aish.generateCommand",
        "title": "Generate Command from Description",
        "category": "AISH"
      }
    ],
    "keybindings": [
      {
        "command": "aish.analyzeTerminal",
        "key": "ctrl+shift+a",
        "when": "terminalFocus"
      }
    ],
    "configuration": {
      "title": "AISH",
      "properties": {
        "aish.provider": {
          "type": "string",
          "default": "gemini-cli",
          "enum": ["openai", "gemini", "gemini-cli"],
          "description": "LLM provider to use"
        },
        "aish.autoAnalyze": {
          "type": "boolean",
          "default": true,
          "description": "Automatically analyze terminal errors"
        }
      }
    }
  }
}
```

```typescript
// src/extension.ts
import * as vscode from 'vscode';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

export function activate(context: vscode.ExtensionContext) {
    // Register command to analyze terminal
    const analyzeCommand = vscode.commands.registerCommand('aish.analyzeTerminal', async () => {
        const terminal = vscode.window.activeTerminal;
        if (!terminal) {
            vscode.window.showErrorMessage('No active terminal found');
            return;
        }

        try {
            // Get the last command from terminal history
            const { stdout } = await execAsync('aish history 1 --json');
            const history = JSON.parse(stdout);

            if (history.length > 0) {
                const lastError = history[0];
                showAnalysisPanel(lastError);
            }
        } catch (error) {
            vscode.window.showErrorMessage(`Failed to analyze terminal: ${error}`);
        }
    });

    // Register command to generate command
    const generateCommand = vscode.commands.registerCommand('aish.generateCommand', async () => {
        const prompt = await vscode.window.showInputBox({
            prompt: 'Describe what you want to do',
            placeHolder: 'e.g., "find all JavaScript files modified today"'
        });

        if (prompt) {
            try {
                const { stdout } = await execAsync(`aish -p "${prompt}" --json`);
                const result = JSON.parse(stdout);

                const action = await vscode.window.showInformationMessage(
                    `Generated command: ${result.command}`,
                    'Copy to Clipboard',
                    'Insert in Terminal'
                );

                if (action === 'Copy to Clipboard') {
                    await vscode.env.clipboard.writeText(result.command);
                } else if (action === 'Insert in Terminal') {
                    const terminal = vscode.window.activeTerminal || vscode.window.createTerminal();
                    terminal.sendText(result.command, false);
                    terminal.show();
                }
            } catch (error) {
                vscode.window.showErrorMessage(`Failed to generate command: ${error}`);
            }
        }
    });

    context.subscriptions.push(analyzeCommand, generateCommand);

    // Auto-analyze terminal errors
    const config = vscode.workspace.getConfiguration('aish');
    if (config.get('autoAnalyze')) {
        vscode.window.onDidOpenTerminal(terminal => {
            // Monitor terminal for errors
            setupTerminalMonitoring(terminal);
        });
    }
}

function showAnalysisPanel(errorData: any) {
    const panel = vscode.window.createWebviewPanel(
        'aishAnalysis',
        'AISH Error Analysis',
        vscode.ViewColumn.Beside,
        {
            enableScripts: true
        }
    );

    panel.webview.html = getAnalysisWebviewContent(errorData);
}

function setupTerminalMonitoring(terminal: vscode.Terminal) {
    // Implementation for monitoring terminal output
    // This would require VS Code API extensions or external monitoring
}

function getAnalysisWebviewContent(errorData: any): string {
    return `
    <!DOCTYPE html>
    <html>
    <head>
        <style>
            body { font-family: Arial, sans-serif; padding: 20px; }
            .error-command { background: #f5f5f5; padding: 10px; border-radius: 5px; }
            .explanation { margin: 15px 0; }
            .suggestion { background: #e8f5e8; padding: 10px; border-radius: 5px; }
        </style>
    </head>
    <body>
        <h2>Error Analysis</h2>
        <div class="error-command">
            <strong>Command:</strong> ${errorData.command}<br>
            <strong>Exit Code:</strong> ${errorData.exitCode}<br>
            <strong>Error:</strong> ${errorData.stderr}
        </div>
        <div class="explanation">
            <h3>Explanation</h3>
            <p>${errorData.analysis?.explanation || 'No analysis available'}</p>
        </div>
        <div class="suggestion">
            <h3>Suggested Fix</h3>
            <code>${errorData.analysis?.correctedCommand || 'No suggestion available'}</code>
        </div>
    </body>
    </html>
    `;
}
```

### Vim/Neovim Plugin

```lua
-- lua/aish.lua
local M = {}

-- Configuration
M.config = {
    provider = "gemini-cli",
    auto_analyze = true,
    keybindings = {
        analyze = "<leader>aa",
        generate = "<leader>ag"
    }
}

-- Analyze terminal error
function M.analyze_error()
    local cmd = "aish history 1 --json"
    local handle = io.popen(cmd)
    local result = handle:read("*a")
    handle:close()

    local ok, history = pcall(vim.json.decode, result)
    if not ok or #history == 0 then
        vim.notify("No recent errors found", vim.log.levels.INFO)
        return
    end

    local error_data = history[1]
    M.show_analysis_popup(error_data)
end

-- Generate command from description
function M.generate_command()
    vim.ui.input({ prompt = "Describe what you want to do: " }, function(prompt)
        if not prompt then return end

        local cmd = string.format('aish -p "%s" --json', prompt)
        local handle = io.popen(cmd)
        local result = handle:read("*a")
        handle:close()

        local ok, data = pcall(vim.json.decode, result)
        if not ok then
            vim.notify("Failed to generate command", vim.log.levels.ERROR)
            return
        end

        vim.ui.select(
            { "Insert at cursor", "Copy to clipboard", "Execute in terminal" },
            { prompt = "Command: " .. data.command },
            function(choice)
                if choice == "Insert at cursor" then
                    vim.api.nvim_put({data.command}, "c", true, true)
                elseif choice == "Copy to clipboard" then
                    vim.fn.setreg("+", data.command)
                elseif choice == "Execute in terminal" then
                    vim.cmd("terminal " .. data.command)
                end
            end
        )
    end)
end

-- Show analysis in popup
function M.show_analysis_popup(error_data)
    local content = {
        "Error Analysis",
        "═══════════════",
        "",
        "Command: " .. error_data.command,
        "Exit Code: " .. error_data.exitCode,
        "Error: " .. (error_data.stderr or ""),
        "",
        "Explanation:",
        error_data.analysis and error_data.analysis.explanation or "No analysis available",
        "",
        "Suggested Fix:",
        error_data.analysis and error_data.analysis.correctedCommand or "No suggestion available"
    }

    local width = math.max(unpack(vim.tbl_map(string.len, content))) + 4
    local height = #content + 2

    local buf = vim.api.nvim_create_buf(false, true)
    vim.api.nvim_buf_set_lines(buf, 0, -1, false, content)

    local opts = {
        style = "minimal",
        relative = "cursor",
        width = width,
        height = height,
        row = 1,
        col = 1,
        border = "rounded"
    }

    vim.api.nvim_open_win(buf, true, opts)
end

-- Setup function
function M.setup(opts)
    M.config = vim.tbl_deep_extend("force", M.config, opts or {})

    -- Create commands
    vim.api.nvim_create_user_command("AishAnalyze", M.analyze_error, {})
    vim.api.nvim_create_user_command("AishGenerate", M.generate_command, {})

    -- Set up keybindings
    vim.keymap.set("n", M.config.keybindings.analyze, M.analyze_error, { desc = "Analyze terminal error with AISH" })
    vim.keymap.set("n", M.config.keybindings.generate, M.generate_command, { desc = "Generate command with AISH" })
end

return M
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/aish-analysis.yml
name: AISH Error Analysis

on:
  workflow_run:
    workflows: ["CI"]
    types:
      - completed

jobs:
  analyze-failures:
    if: ${{ github.event.workflow_run.conclusion == 'failure' }}
    runs-on: ubuntu-latest

    steps:
    - name: Checkout code
      uses: actions/checkout@v4

    - name: Install AISH
      run: |
        curl -sSL https://raw.githubusercontent.com/TonnyWong1052/aish/main/scripts/install.sh | bash
        echo "$HOME/bin" >> $GITHUB_PATH

    - name: Configure AISH
      env:
        GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
      run: |
        aish config set default_provider gemini
        aish config set providers.gemini.api_key "$GEMINI_API_KEY"

    - name: Download workflow logs
      uses: actions/github-script@v7
      with:
        script: |
          const fs = require('fs');
          const { data: logs } = await github.rest.actions.downloadWorkflowRunLogs({
            owner: context.repo.owner,
            repo: context.repo.repo,
            run_id: ${{ github.event.workflow_run.id }}
          });
          fs.writeFileSync('workflow-logs.zip', Buffer.from(logs));

    - name: Extract and analyze logs
      run: |
        unzip workflow-logs.zip

        # Find failed jobs and analyze with AISH
        for log_file in */*.txt; do
          if grep -q "Error\|Failed\|Exception" "$log_file"; then
            echo "Analyzing failures in $log_file"

            # Extract error context
            grep -A 5 -B 5 "Error\|Failed\|Exception" "$log_file" > error_context.txt

            # Use AISH to analyze
            aish -p "Analyze this CI/CD error and suggest fixes: $(cat error_context.txt)" >> analysis_results.md
            echo "---" >> analysis_results.md
          fi
        done

    - name: Create issue with analysis
      uses: actions/github-script@v7
      with:
        script: |
          const fs = require('fs');
          const analysisResults = fs.readFileSync('analysis_results.md', 'utf8');

          await github.rest.issues.create({
            owner: context.repo.owner,
            repo: context.repo.repo,
            title: `CI/CD Failure Analysis - ${new Date().toISOString().split('T')[0]}`,
            body: `## Automated Failure Analysis\n\nWorkflow: ${{ github.event.workflow_run.name }}\nRun ID: ${{ github.event.workflow_run.id }}\n\n${analysisResults}`,
            labels: ['ci-failure', 'automated-analysis']
          });
```

### GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - test
  - analyze

variables:
  AISH_CONFIG_DIR: "${CI_PROJECT_DIR}/.aish"

test:
  stage: test
  script:
    - make test
  artifacts:
    when: on_failure
    reports:
      junit: test-results.xml
    paths:
      - test-output.log
    expire_in: 1 hour

analyze_failures:
  stage: analyze
  image: alpine:latest
  when: on_failure
  dependencies:
    - test
  before_script:
    - apk add --no-cache curl bash
    - curl -sSL https://raw.githubusercontent.com/TonnyWong1052/aish/main/scripts/install.sh | bash
    - export PATH="$HOME/bin:$PATH"
    - mkdir -p $AISH_CONFIG_DIR
    - aish config set default_provider gemini
    - aish config set providers.gemini.api_key "$GEMINI_API_KEY"
  script:
    - |
      if [ -f test-output.log ]; then
        echo "Analyzing test failures..."
        aish -p "Analyze this test failure and suggest fixes: $(cat test-output.log)" > failure-analysis.md

        # Post to merge request
        if [ -n "$CI_MERGE_REQUEST_IID" ]; then
          curl --header "PRIVATE-TOKEN: $GITLAB_TOKEN" \
               --header "Content-Type: application/json" \
               --data "{\"body\": \"## Automated Failure Analysis\\n\\n$(cat failure-analysis.md)\"}" \
               "$CI_API_V4_URL/projects/$CI_PROJECT_ID/merge_requests/$CI_MERGE_REQUEST_IID/notes"
        fi
      fi
  artifacts:
    paths:
      - failure-analysis.md
    expire_in: 1 week
```

## Container Integration

### Docker Integration

```dockerfile
# Dockerfile.aish
FROM alpine:latest

# Install dependencies
RUN apk add --no-cache \
    bash \
    curl \
    ca-certificates \
    && rm -rf /var/cache/apk/*

# Install AISH
RUN curl -sSL https://raw.githubusercontent.com/TonnyWong1052/aish/main/scripts/install.sh | bash
ENV PATH="/root/bin:${PATH}"

# Configure AISH for container usage
RUN mkdir -p /root/.config/aish
COPY aish-config.json /root/.config/aish/config.json

# Set up entrypoint
COPY docker-entrypoint.sh /usr/local/bin/
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["aish", "--help"]
```

```bash
#!/bin/bash
# docker-entrypoint.sh

set -e

# Initialize AISH if not configured
if [ ! -f "/root/.config/aish/config.json" ]; then
    if [ -n "$GEMINI_API_KEY" ]; then
        aish config set default_provider gemini
        aish config set providers.gemini.api_key "$GEMINI_API_KEY"
    elif [ -n "$OPENAI_API_KEY" ]; then
        aish config set default_provider openai
        aish config set providers.openai.api_key "$OPENAI_API_KEY"
    fi
fi

# Execute the requested command
exec "$@"
```

### Kubernetes Job

```yaml
# k8s-aish-analyzer.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: aish-log-analyzer
  namespace: monitoring
spec:
  template:
    spec:
      containers:
      - name: aish-analyzer
        image: your-registry/aish:latest
        env:
        - name: GEMINI_API_KEY
          valueFrom:
            secretKeyRef:
              name: aish-secrets
              key: gemini-api-key
        command: ["/bin/bash"]
        args:
        - -c
        - |
          # Fetch logs from failed pods
          kubectl get pods --field-selector=status.phase=Failed -o json | \
          jq -r '.items[].metadata.name' | \
          while read pod; do
            echo "Analyzing pod: $pod"
            kubectl logs $pod > /tmp/pod-logs.txt
            aish -p "Analyze this Kubernetes pod failure: $(cat /tmp/pod-logs.txt)" >> /tmp/analysis.md
            echo "---" >> /tmp/analysis.md
          done

          # Create ConfigMap with analysis results
          kubectl create configmap aish-analysis-$(date +%Y%m%d-%H%M%S) --from-file=/tmp/analysis.md
        volumeMounts:
        - name: kubectl-config
          mountPath: /root/.kube
      volumes:
      - name: kubectl-config
        secret:
          secretName: kubectl-config
      restartPolicy: OnFailure
```

## Cloud Platform Integration

### AWS Lambda Function

```python
# lambda_aish_analyzer.py
import json
import subprocess
import boto3
import os
from datetime import datetime

def lambda_handler(event, context):
    """
    Lambda function to analyze CloudWatch logs with AISH
    """

    # Initialize clients
    logs_client = boto3.client('logs')
    sns_client = boto3.client('sns')

    # Get log group and stream from event
    log_group = event.get('logGroup')
    log_stream = event.get('logStream')

    if not log_group or not log_stream:
        return {
            'statusCode': 400,
            'body': json.dumps('Missing logGroup or logStream')
        }

    try:
        # Fetch logs from CloudWatch
        response = logs_client.get_log_events(
            logGroupName=log_group,
            logStreamName=log_stream,
            startFromHead=False,
            limit=100
        )

        # Extract error messages
        error_logs = []
        for event in response['events']:
            message = event['message']
            if any(keyword in message.lower() for keyword in ['error', 'exception', 'failed', 'timeout']):
                error_logs.append(message)

        if not error_logs:
            return {
                'statusCode': 200,
                'body': json.dumps('No errors found in logs')
            }

        # Analyze with AISH
        error_context = '\n'.join(error_logs[-10:])  # Last 10 errors

        # Configure AISH
        os.environ['AISH_CONFIG_DIR'] = '/tmp/.aish'
        subprocess.run(['mkdir', '-p', '/tmp/.aish'], check=True)

        subprocess.run([
            'aish', 'config', 'set', 'default_provider', 'gemini'
        ], check=True)

        subprocess.run([
            'aish', 'config', 'set', 'providers.gemini.api_key',
            os.environ['GEMINI_API_KEY']
        ], check=True)

        # Run analysis
        result = subprocess.run([
            'aish', '-p',
            f'Analyze these AWS Lambda/CloudWatch errors and suggest fixes: {error_context}'
        ], capture_output=True, text=True, check=True)

        analysis = result.stdout

        # Send notification
        message = f"""
        AWS Error Analysis Report
        ========================

        Log Group: {log_group}
        Log Stream: {log_stream}
        Time: {datetime.now().isoformat()}

        Analysis:
        {analysis}

        Error Context:
        {error_context}
        """

        sns_client.publish(
            TopicArn=os.environ['SNS_TOPIC_ARN'],
            Subject='AWS Error Analysis Report',
            Message=message
        )

        return {
            'statusCode': 200,
            'body': json.dumps({
                'analysis': analysis,
                'errorCount': len(error_logs)
            })
        }

    except Exception as e:
        return {
            'statusCode': 500,
            'body': json.dumps(f'Error analyzing logs: {str(e)}')
        }

# requirements.txt
# boto3==1.34.0
# requests==2.31.0
```

### Google Cloud Function

```javascript
// index.js
const { execSync } = require('child_process');
const { Storage } = require('@google-cloud/storage');
const { PubSub } = require('@google-cloud/pubsub');

const storage = new Storage();
const pubsub = new PubSub();

exports.analyzeCloudLogs = async (req, res) => {
    try {
        const { bucketName, fileName } = req.body;

        if (!bucketName || !fileName) {
            return res.status(400).send('Missing bucketName or fileName');
        }

        // Download log file from Cloud Storage
        const bucket = storage.bucket(bucketName);
        const file = bucket.file(fileName);

        const [contents] = await file.download();
        const logData = contents.toString();

        // Filter for errors
        const errorLines = logData
            .split('\n')
            .filter(line => /error|exception|failed|timeout/i.test(line))
            .slice(-20); // Last 20 errors

        if (errorLines.length === 0) {
            return res.status(200).json({ message: 'No errors found' });
        }

        // Configure AISH
        const configDir = '/tmp/.aish';
        execSync(`mkdir -p ${configDir}`);
        execSync(`AISH_CONFIG_DIR=${configDir} aish config set default_provider gemini`);
        execSync(`AISH_CONFIG_DIR=${configDir} aish config set providers.gemini.api_key "${process.env.GEMINI_API_KEY}"`);

        // Analyze with AISH
        const errorContext = errorLines.join('\n');
        const analysisCommand = `AISH_CONFIG_DIR=${configDir} aish -p "Analyze these Google Cloud errors and suggest fixes: ${errorContext.replace(/"/g, '\\"')}"`;

        const analysis = execSync(analysisCommand, { encoding: 'utf8' });

        // Publish results to Pub/Sub
        const message = {
            timestamp: new Date().toISOString(),
            bucketName,
            fileName,
            errorCount: errorLines.length,
            analysis,
            errorContext
        };

        const messageBuffer = Buffer.from(JSON.stringify(message));
        await pubsub.topic('aish-analysis-results').publish(messageBuffer);

        res.status(200).json({
            analysis,
            errorCount: errorLines.length
        });

    } catch (error) {
        console.error('Error analyzing logs:', error);
        res.status(500).send(`Error analyzing logs: ${error.message}`);
    }
};

// package.json
{
  "name": "aish-cloud-analyzer",
  "version": "1.0.0",
  "dependencies": {
    "@google-cloud/storage": "^7.0.0",
    "@google-cloud/pubsub": "^4.0.0"
  }
}
```

## Custom Tool Integration

### Monitoring Integration (Prometheus/Grafana)

```python
#!/usr/bin/env python3
# aish_prometheus_exporter.py

import time
import json
import subprocess
import requests
from prometheus_client import start_http_server, Gauge, Counter, Histogram

# Metrics
aish_analysis_requests = Counter('aish_analysis_requests_total', 'Total AISH analysis requests')
aish_analysis_duration = Histogram('aish_analysis_duration_seconds', 'Time spent on AISH analysis')
aish_error_classifications = Counter('aish_error_classifications_total', 'Error classifications by type', ['error_type'])
aish_provider_requests = Counter('aish_provider_requests_total', 'Requests by provider', ['provider'])

class AishMonitor:
    def __init__(self, aish_config_path="/home/user/.config/aish"):
        self.config_path = aish_config_path

    def get_aish_stats(self):
        """Get AISH usage statistics"""
        try:
            result = subprocess.run([
                'aish', 'stats', '--json'
            ], capture_output=True, text=True, check=True)

            return json.loads(result.stdout)
        except subprocess.CalledProcessError:
            return {}

    def get_error_history(self):
        """Get recent error history for classification metrics"""
        try:
            result = subprocess.run([
                'aish', 'history', '--json', '--limit', '100'
            ], capture_output=True, text=True, check=True)

            return json.loads(result.stdout)
        except subprocess.CalledProcessError:
            return []

    def update_metrics(self):
        """Update Prometheus metrics"""
        # Get AISH statistics
        stats = self.get_aish_stats()

        if 'total_requests' in stats:
            aish_analysis_requests._value._value = stats['total_requests']

        if 'provider_usage' in stats:
            for provider, count in stats['provider_usage'].items():
                aish_provider_requests.labels(provider=provider)._value._value = count

        # Update error classification metrics
        history = self.get_error_history()
        error_type_counts = {}

        for entry in history:
            error_type = entry.get('error_type', 'unknown')
            error_type_counts[error_type] = error_type_counts.get(error_type, 0) + 1

        for error_type, count in error_type_counts.items():
            aish_error_classifications.labels(error_type=error_type)._value._value = count

def main():
    monitor = AishMonitor()

    # Start Prometheus metrics server
    start_http_server(8000)
    print("AISH Prometheus exporter started on port 8000")

    while True:
        try:
            monitor.update_metrics()
            time.sleep(30)  # Update every 30 seconds
        except KeyboardInterrupt:
            break
        except Exception as e:
            print(f"Error updating metrics: {e}")
            time.sleep(30)

if __name__ == '__main__':
    main()
```

### Slack Integration

```python
# aish_slack_bot.py
import os
import json
import subprocess
from slack_bolt import App
from slack_bolt.adapter.socket_mode import SocketModeHandler

# Initialize Slack app
app = App(token=os.environ["SLACK_BOT_TOKEN"])

@app.command("/aish-analyze")
def handle_analyze_command(ack, command, client):
    ack()

    try:
        # Get command text (error description or log snippet)
        error_text = command['text']

        if not error_text:
            client.chat_postMessage(
                channel=command['channel_id'],
                text="Please provide an error description or log snippet to analyze."
            )
            return

        # Configure AISH
        config_env = os.environ.copy()
        config_env['AISH_CONFIG_DIR'] = '/tmp/.aish'

        subprocess.run(['mkdir', '-p', '/tmp/.aish'], check=True)
        subprocess.run([
            'aish', 'config', 'set', 'default_provider', 'gemini'
        ], env=config_env, check=True)

        subprocess.run([
            'aish', 'config', 'set', 'providers.gemini.api_key',
            os.environ['GEMINI_API_KEY']
        ], env=config_env, check=True)

        # Analyze with AISH
        result = subprocess.run([
            'aish', '-p', f'Analyze this error and suggest fixes: {error_text}'
        ], env=config_env, capture_output=True, text=True, check=True)

        analysis = result.stdout

        # Send analysis back to Slack
        client.chat_postMessage(
            channel=command['channel_id'],
            blocks=[
                {
                    "type": "section",
                    "text": {
                        "type": "mrkdwn",
                        "text": f"*AISH Error Analysis*\n\n*Original Error:*\n```{error_text}```"
                    }
                },
                {
                    "type": "section",
                    "text": {
                        "type": "mrkdwn",
                        "text": f"*Analysis & Suggestions:*\n{analysis}"
                    }
                }
            ]
        )

    except subprocess.CalledProcessError as e:
        client.chat_postMessage(
            channel=command['channel_id'],
            text=f"Error running AISH analysis: {e.stderr}"
        )
    except Exception as e:
        client.chat_postMessage(
            channel=command['channel_id'],
            text=f"Unexpected error: {str(e)}"
        )

@app.command("/aish-generate")
def handle_generate_command(ack, command, client):
    ack()

    try:
        prompt_text = command['text']

        if not prompt_text:
            client.chat_postMessage(
                channel=command['channel_id'],
                text="Please provide a description of what command you want to generate."
            )
            return

        # Configure AISH
        config_env = os.environ.copy()
        config_env['AISH_CONFIG_DIR'] = '/tmp/.aish'

        # Generate command with AISH
        result = subprocess.run([
            'aish', '-p', prompt_text, '--json'
        ], env=config_env, capture_output=True, text=True, check=True)

        data = json.loads(result.stdout)
        generated_command = data.get('command', result.stdout.strip())

        # Send generated command back to Slack
        client.chat_postMessage(
            channel=command['channel_id'],
            blocks=[
                {
                    "type": "section",
                    "text": {
                        "type": "mrkdwn",
                        "text": f"*Generated Command*\n\n*Prompt:* {prompt_text}\n*Command:*\n```{generated_command}```"
                    }
                }
            ]
        )

    except Exception as e:
        client.chat_postMessage(
            channel=command['channel_id'],
            text=f"Error generating command: {str(e)}"
        )

if __name__ == "__main__":
    handler = SocketModeHandler(app, os.environ["SLACK_APP_TOKEN"])
    handler.start()
```

## Webhook Integration

### Generic Webhook Receiver

```python
# aish_webhook_server.py
from flask import Flask, request, jsonify
import subprocess
import json
import os
import hmac
import hashlib

app = Flask(__name__)

def verify_signature(payload, signature, secret):
    """Verify webhook signature"""
    if not signature:
        return False

    expected_signature = 'sha256=' + hmac.new(
        secret.encode('utf-8'),
        payload,
        hashlib.sha256
    ).hexdigest()

    return hmac.compare_digest(signature, expected_signature)

@app.route('/webhook/analyze', methods=['POST'])
def analyze_webhook():
    """Generic webhook endpoint for error analysis"""

    # Verify signature if secret is configured
    webhook_secret = os.environ.get('WEBHOOK_SECRET')
    if webhook_secret:
        signature = request.headers.get('X-Hub-Signature-256')
        if not verify_signature(request.data, signature, webhook_secret):
            return jsonify({'error': 'Invalid signature'}), 401

    try:
        data = request.json

        # Extract error information from webhook payload
        error_data = {
            'command': data.get('command', ''),
            'error_output': data.get('error', ''),
            'exit_code': data.get('exit_code', 1),
            'context': data.get('context', {})
        }

        # Configure AISH
        config_env = os.environ.copy()
        config_env['AISH_CONFIG_DIR'] = '/tmp/.aish'

        subprocess.run(['mkdir', '-p', '/tmp/.aish'], check=True)
        subprocess.run([
            'aish', 'config', 'set', 'default_provider', 'gemini'
        ], env=config_env, check=True)

        subprocess.run([
            'aish', 'config', 'set', 'providers.gemini.api_key',
            os.environ['GEMINI_API_KEY']
        ], env=config_env, check=True)

        # Prepare analysis prompt
        context_str = f"""
        Command: {error_data['command']}
        Error Output: {error_data['error_output']}
        Exit Code: {error_data['exit_code']}
        Additional Context: {json.dumps(error_data['context'])}
        """

        # Analyze with AISH
        result = subprocess.run([
            'aish', '-p', f'Analyze this error and suggest fixes: {context_str}'
        ], env=config_env, capture_output=True, text=True, check=True)

        analysis = result.stdout.strip()

        # Return analysis
        response = {
            'status': 'success',
            'analysis': analysis,
            'error_data': error_data
        }

        # Optional: Forward to callback URL
        callback_url = data.get('callback_url')
        if callback_url:
            import requests
            requests.post(callback_url, json=response, timeout=10)

        return jsonify(response)

    except subprocess.CalledProcessError as e:
        return jsonify({
            'status': 'error',
            'message': f'AISH analysis failed: {e.stderr}'
        }), 500
    except Exception as e:
        return jsonify({
            'status': 'error',
            'message': f'Unexpected error: {str(e)}'
        }), 500

@app.route('/webhook/health', methods=['GET'])
def health_check():
    """Health check endpoint"""
    return jsonify({'status': 'healthy', 'service': 'aish-webhook'})

if __name__ == '__main__':
    port = int(os.environ.get('PORT', 8080))
    app.run(host='0.0.0.0', port=port, debug=False)
```

### Usage Examples

```bash
# Send webhook request
curl -X POST http://localhost:8080/webhook/analyze \
  -H "Content-Type: application/json" \
  -H "X-Hub-Signature-256: sha256=..." \
  -d '{
    "command": "docker run nginx",
    "error": "docker: Error response from daemon: port 80 already in use",
    "exit_code": 1,
    "context": {
      "platform": "linux",
      "docker_version": "24.0.0"
    },
    "callback_url": "https://your-service.com/aish-callback"
  }'
```

This integration guide provides comprehensive examples for integrating AISH with various systems and workflows. Each integration can be customized based on specific requirements and environments.