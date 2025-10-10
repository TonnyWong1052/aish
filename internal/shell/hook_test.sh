#!/usr/bin/env bats

# This file tests the shell hook logic using the BATS testing framework.
# It simulates a shell environment and verifies the behavior of the hook.

# Load the hook script to be tested
# We need to source it so its functions are available in the test environment.
# The path is relative to this test file's location.
HOOK_SCRIPT_PATH="../internal/shell/assets/hook.sh"

# Setup: This runs before each test.
setup() {
    # Create a temporary directory for state files
    export AISH_STATE_DIR="${BATS_TMPDIR}/aish_test_state"
    mkdir -p "$AISH_STATE_DIR"

    # Define paths for state files based on the hook script's logic
    export AISH_STDOUT_FILE="$AISH_STATE_DIR/last_stdout"
    export AISH_STDERR_FILE="$AISH_STATE_DIR/last_stderr"
    export AISH_LAST_CMD_FILE="$AISH_STATE_DIR/last_command"

    # Source the hook script to make its functions available
    # shellcheck source=../internal/shell/assets/hook.sh
    source "$HOOK_SCRIPT_PATH"

    # Unset any variables that might interfere with tests
    unset AISH_HOOK_DISABLED
    unset AISH_CAPTURE_OFF
    unset AISH_SKIP_COMMAND_PATTERNS
    unset AISH_SKIP_ALL_USER_COMMANDS
    unset AISH_SYSTEM_DIR_WHITELIST
    unset ZSH_VERSION
    unset BASH_VERSION
    unset PROMPT_COMMAND
    unset __aish_capture_on
}

# Teardown: This runs after each test.
teardown() {
    # Clean up the temporary directory
    rm -rf "$AISH_STATE_DIR"
}

# --- Test Helper Functions ---

# Simulates the pre-execution part of the hook.
# It sets up state files and redirections.
simulate_preexec() {
    local cmd="$1"
    # Ensure the command is sanitized and saved
    __aish_sanitize_cmd "$cmd" > "$AISH_LAST_CMD_FILE"
    # Create dummy stdout/stderr files for the command to write to
    : > "$AISH_STDOUT_FILE"
    : > "$AISH_STDERR_FILE"
    # In a real hook, FD redirection happens here.
    # For testing, we'll manually write to these files.
}

# Simulates the post-execution part of the hook.
# It checks conditions and calls the 'aish capture' command.
# Returns 0 if 'aish capture' would be called, 1 otherwise.
simulate_precmd() {
    local exit_code="$1"
    local last_command
    last_command=$(cat "$AISH_LAST_CMD_FILE" 2>/dev/null)

    if [ "$exit_code" -ne 0 ] && [ -n "$last_command" ]; then
        if __aish_should_trigger "$exit_code"; then
            if ! __aish_should_skip_cmd "$last_command"; then
                # In a real scenario, this would call 'aish capture'
                # For testing, we'll just echo a success marker.
                echo "aish_capture_called"
                return 0
            fi
        fi
    fi
    return 1
}


# --- Test Cases ---

@test "__aish_sanitize_cmd masks API keys" {
    run __aish_sanitize_cmd "curl -H 'Authorization: Bearer mysecrettoken' https://api.example.com"
    [ "$status" -eq 0 ]
    [[ "$output" == *"Authorization: Bearer ***REDACTED***"* ]]
}

@test "__aish_sanitize_cmd masks password flags" {
    run __aish_sanitize_cmd "mysql -u root -p'password123' db"
    [ "$status" -eq 0 ]
    [[ "$output" == *"-p'***REDACTED***'"* ]]
}

@test "__aish_sanitize_cmd masks environment variables" {
    run __aish_sanitize_cmd "DB_PASSWORD=secret psql -l"
    [ "$status" -eq 0 ]
    [[ "$output" == *"DB_PASSWORD=***REDACTED***"* ]]
}

@test "__aish_should_trigger returns 1 for exit code 0" {
    run __aish_should_trigger 0
    [ "$status" -eq 1 ]
}

@test "__aish_should_trigger returns 1 for SIGINT (130)" {
    run __aish_should_trigger 130
    [ "$status" -eq 1 ]
}

@test "__aish_should_trigger returns 1 for SIGQUIT (131)" {
    run __aish_should_trigger 131
    [ "$status" -eq 1 ]
}

@test "__aish_should_trigger returns 0 for 'command not found'" {
    echo "command not found: fakecommand" > "$AISH_STDERR_FILE"
    run __aish_should_trigger 127 # Common exit code for command not found
    [ "$status" -eq 0 ]
}

@test "__aish_should_trigger returns 0 for 'No such file or directory'" {
    echo "cat: /nonexistent/file: No such file or directory" > "$AISH_STDERR_FILE"
    run __aish_should_trigger 1
    [ "$status" -eq 0 ]
}

@test "__aish_should_trigger returns 0 for empty stderr (conservative mode)" {
    : > "$AISH_STDERR_FILE" # Create empty stderr
    run __aish_should_trigger 1
    [ "$status" -eq 0 ]
}

@test "__aish_should_skip_cmd returns 0 for aish command" {
    run __aish_should_skip_cmd "aish configure"
    [ "$status" -eq 0 ]
}

@test "__aish_should_skip_cmd returns 0 for npm command" {
    run __aish_should_skip_cmd "npm install"
    [ "$status" -eq 0 ]
}

@test "__aish_should_skip_cmd returns 1 for a regular command" {
    run __aish_should_skip_cmd "ls -l"
    [ "$status" -eq 1 ] # Should not skip
}

@test "__aish_should_skip_cmd returns 0 for user-defined pattern" {
    export AISH_SKIP_COMMAND_PATTERNS="mytool*"
    run __aish_should_skip_cmd "mytool run"
    [ "$status" -eq 0 ]
}

@test "__aish_should_skip_cmd skips user-installed command when enabled" {
    # This test requires mocking `command -v` or ensuring a test binary exists
    # For simplicity, we'll assume a command like `myusercmd` is in /usr/local/bin
    export AISH_SKIP_ALL_USER_COMMANDS="1"
    export AISH_SYSTEM_DIR_WHITELIST="/bin:/usr/bin"
    # Mock command -v to return a user path
    command() { [ "$1" = "-v" ] && echo "/usr/local/bin/myusercmd"; }
    run __aish_should_skip_cmd "myusercmd"
    [ "$status" -eq 0 ]
    unset -f command # Unset the mock
}

@test "Full flow: command not found triggers capture" {
    simulate_preexec "fakecommand"
    echo "bash: fakecommand: command not found" > "$AISH_STDERR_FILE"
    run simulate_precmd 127
    [ "$status" -eq 0 ]
    [ "$output" = "aish_capture_called" ]
}

@test "Full flow: successful command does not trigger capture" {
    simulate_preexec "ls"
    echo "file1 file2" > "$AISH_STDOUT_FILE"
    run simulate_precmd 0
    [ "$status" -eq 1 ] # Should not trigger
    [ "$output" = "" ]
}

@test "Full flow: skipped command does not trigger capture" {
    simulate_preexec "npm install"
    echo "npm ERR! something went wrong" > "$AISH_STDERR_FILE"
    run simulate_precmd 1
    [ "$status" -eq 1 ] # Should not trigger
    [ "$output" = "" ]
}

# Note: Testing the actual hook installation (addHookToFile, removeHookFromFile)
# and the complex FD redirection/restore logic is more suited for
# integration tests or would require significant mocking of file systems
# and shell state. These BATS tests focus on the core logic of the
# hook functions themselves.