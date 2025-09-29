# AISH (AI Shell) Hook - Start

# State file locations
if [ -z "$AISH_STATE_DIR" ]; then
    AISH_STATE_DIR="$HOME/.config/aish"
fi
AISH_STDOUT_FILE="$AISH_STATE_DIR/last_stdout"
AISH_STDERR_FILE="$AISH_STATE_DIR/last_stderr"
AISH_LAST_CMD_FILE="$AISH_STATE_DIR/last_command"
mkdir -p "$AISH_STATE_DIR" >/dev/null 2>&1 || true

# Load user preferences if present
if [ -f "$AISH_STATE_DIR/env.sh" ]; then
    . "$AISH_STATE_DIR/env.sh" >/dev/null 2>&1 || true
fi

# 預設：跳過所有非系統路徑的使用者安裝命令（可用 AISH_SKIP_ALL_USER_COMMANDS=0 覆寫）
if [ -z "${AISH_SKIP_ALL_USER_COMMANDS+x}" ]; then
    AISH_SKIP_ALL_USER_COMMANDS="1"
fi

# Sensitive information masking: replace common sensitive parameter values in commands with ***REDACTED***
__aish_sanitize_cmd() {
    local _c="$1"
    # Flag form --key=VALUE / --key VALUE (use double quotes to avoid nested quote issues)
    _c=$(printf "%s" "$_c" | sed -E "s/--(api[_-]?key|token|password|passwd|secret|bearer)=([^[:space:]]+)/--\\1=***REDACTED***/g")
    _c=$(printf "%s" "$_c" | sed -E "s/--(api[_-]?key|token|password|passwd|secret|bearer)[[:space:]]+([^[:space:]]+)/--\\1 ***REDACTED***/g")
    # Environment variable form FOO_TOKEN=VALUE or ...SECRET...=VALUE
    _c=$(printf "%s" "$_c" | sed -E "s/([A-Za-z_][A-Za-z0-9_]*((SECRET)|(TOKEN)|(PASSWORD)|(API[_-]?KEY)|(ACCESS[_-]?KEY)|(BEARER))[A-Za-z0-9_]*)=([^[:space:]]+)/\\1=***REDACTED***/g")
    echo "$_c"
}

# Animation function to show loading spinner with elapsed seconds in background.
# Runs a lightweight background loop that updates a single line on /dev/tty.
# zsh uses '&!' to start already-disowned jobs; bash uses '&' then 'disown %%'.
__aish_show_loading_animation() {
    local message="${1:-Generating command...}"
    local start_ts
    start_ts=$(date +%s 2>/dev/null || printf '%s' 0)
    # Frames for the spinner animation
    local frames=("▁" "▃" "▄" "▅" "▆" "▇" "▆" "▅" "▄" "▃")

    if [ -n "$ZSH_VERSION" ]; then
        (
            setopt localoptions nomonitor 2>/dev/null || true
            local i=0 now elapsed frame
            while :; do
                now=$(date +%s 2>/dev/null || printf '%s' 0)
                elapsed=$(( now - start_ts ))
                frame=${frames[$(( i % ${#frames[@]} ))]}
                if [ -w /dev/tty ]; then
                    printf "\r\033[K %s %s (%ss)" "$frame" "$message" "$elapsed" > /dev/tty
                else
                    printf "\r\033[K %s %s (%ss)" "$frame" "$message" "$elapsed"
                fi
                i=$(( (i + 1) % ${#frames[@]} ))
                sleep 0.1
            done
        ) >/dev/null 2>&1 &!
        AISH_ANIMATION_PID=$!
    else
        (
            set +m 2>/dev/null || true
            local i=0 now elapsed frame
            while :; do
                now=$(date +%s 2>/dev/null || printf '%s' 0)
                elapsed=$(( now - start_ts ))
                frame=${frames[$(( i % ${#frames[@]} ))]}
                if [ -w /dev/tty ]; then
                    printf "\r\033[K %s %s (%ss)" "$frame" "$message" "$elapsed" > /dev/tty
                else
                    printf "\r\033[K %s %s (%ss)" "$frame" "$message" "$elapsed"
                fi
                i=$(( (i + 1) % ${#frames[@]} ))
                sleep 0.1
            done
        ) >/dev/null 2>&1 &
        AISH_ANIMATION_PID=$!
        # Remove from job table immediately to avoid job-control messages
        disown %% 2>/dev/null || true
    fi
}

# Stop the loading animation and clear the line
__aish_stop_loading_animation() {
    if [ -n "$AISH_ANIMATION_PID" ]; then
        { kill "$AISH_ANIMATION_PID" 2>/dev/null && wait "$AISH_ANIMATION_PID" 2>/dev/null; } >/dev/null 2>&1
        AISH_ANIMATION_PID=""
    fi
    if [ -w /dev/tty ]; then
        printf "\r\033[K" > /dev/tty
    else
        printf "\r\033[K"
    fi
}

# Common error keywords for pre-filtering on hook side to reduce invalid triggers
__aish_should_trigger() {
    local exit_code="$1"
    # Only consider non-zero exit codes
    if [ "$exit_code" -eq 0 ]; then
        return 1
    fi
    # Skip user-initiated cancellation/termination (Ctrl+C=SIGINT=130, Ctrl+\=SIGQUIT=131, SIGTERM=143)
    if [ "$exit_code" -eq 130 ] || [ "$exit_code" -eq 131 ] || [ "$exit_code" -eq 143 ]; then
        return 1
    fi
    # If stderr doesn't exist, still allow reporting, let application side filter (conservative mode)
    if [ ! -s "$AISH_STDERR_FILE" ]; then
        return 0
    fi
    # Align with classifier keywords for preliminary judgment
    if grep -Eiq '(command not found|No such file or directory|Permission denied|cannot execute binary file|invalid (argument|option)|File exists|is not a directory)' "$AISH_STDERR_FILE"; then
        return 0
    fi
    return 1
}

# General: avoid recursive triggering and known interactive commands
__aish_should_skip_cmd() {
    local _raw="$1"
    local _first="${_raw%% *}"

    case "$_first" in
        ""|aish*|*/aish*)
            return 0
            ;;
    esac

    # Built-in skip list for interactive tools that misbehave under tee redirection
    case "$_first" in
        claude|*/claude|npm|*/npm|npx|*/npx|brew|*/brew|yarn|*/yarn|pnpm|*/pnpm)
            return 0
            ;;
    esac

    # User-defined skip patterns (whitespace separated globs)
    if [ -n "$AISH_SKIP_COMMAND_PATTERNS" ]; then
        for pattern in $AISH_SKIP_COMMAND_PATTERNS; do
            case "$_first" in
                $pattern)
                    return 0
                    ;;
            esac
            case "$_raw" in
                $pattern)
                    return 0
                    ;;
            esac
        done
    fi

    # Skip all user-installed commands when enabled
    if [ "${AISH_SKIP_ALL_USER_COMMANDS:-0}" = "1" ]; then
        local _resolved=""
        if [[ "$_first" == */* ]]; then
            _resolved="$_first"
        else
            _resolved="$(command -v -- "$_first" 2>/dev/null || true)"
        fi
        # If not an absolute path (builtin/alias), keep capture
        case "$_resolved" in
            /*) ;;
            *) return 1;;
        esac
        # System directories whitelist (colon-separated)
        local _wl="${AISH_SYSTEM_DIR_WHITELIST:-/bin:/usr/bin:/sbin:/usr/sbin:/usr/libexec:/System/Library:/lib:/usr/lib}"
        local _is_system=1
        # 在 zsh 下啟用 shwordsplit，使 IFS 分割生效；在 bash 下無害
        if [ -n "$ZSH_VERSION" ]; then
            setopt localoptions shwordsplit 2>/dev/null || true
        fi
        local _oldIFS="$IFS"; IFS=:
        for d in $_wl; do
            case "$_resolved" in
                "$d"/*) _is_system=0; break;;
            esac
        done
        IFS="$_oldIFS"
        # If command path is NOT under system dirs, skip capture
        if [ "$_is_system" -ne 0 ]; then
            return 0
        fi
    fi

    return 1
}

if [ "${AISH_HOOK_DISABLED:-0}" != "1" ]; then

    if [ -n "$ZSH_VERSION" ]; then
        # zsh version: use preexec/precmd for pre/post wrapping
        __aish_capture_on=0

        _aish_preexec() {
            local cmd="$1"
            # 關閉當前函式範圍的作業控制，避免在管線/程序替換產生 tee 背景程序時顯示如 [2] 16188
            setopt localoptions nomonitor 2>/dev/null || true
            # 若屬於需跳過的指令，清空 last_command 並直接返回，避免 postcmd 以舊值誤觸發
            if __aish_should_skip_cmd "$cmd"; then
                : > "$AISH_LAST_CMD_FILE"
                return
            fi
            # Allow per-invocation bypass
            if [ -n "$AISH_CAPTURE_OFF" ]; then
                return
            fi
            : > "$AISH_STDOUT_FILE"
            : > "$AISH_STDERR_FILE"
            local _sanitized
            _sanitized="$(__aish_sanitize_cmd "$cmd")"
            printf "%s" "$_sanitized" > "$AISH_LAST_CMD_FILE"
            # Save original FD and redirect
            exec 4>&1 5>&2
            exec 1> >(tee -a "$AISH_STDOUT_FILE") 2> >(tee -a "$AISH_STDERR_FILE" >&2)
            __aish_capture_on=1
        }

        _aish_precmd() {
            local exit_code=$?
            local _had_capture=0
            # 同步關閉作業控制訊息，避免在 Ctrl+C 後殘留背景工作提示
            setopt localoptions nomonitor 2>/dev/null || true
            if [ "$__aish_capture_on" = "1" ]; then
                # Restore FD
                exec 1>&4 4>&- 2>&5 5>&-
                __aish_capture_on=0
                _had_capture=1
            fi
            local last_command
            last_command=$(cat "$AISH_LAST_CMD_FILE" 2>/dev/null)
            # 僅在本次指令確實啟用過捕捉時，才嘗試觸發 aish capture，避免 skip 指令誤觸發
            if [ $_had_capture -eq 1 ] && [ $exit_code -ne 0 ] && [ -n "$last_command" ] && command -v aish >/dev/null 2>&1; then
                __aish_should_trigger "$exit_code" || return $exit_code
                __aish_should_skip_cmd "$last_command" && return
                AISH_STDOUT_FILE="$AISH_STDOUT_FILE" AISH_STDERR_FILE="$AISH_STDERR_FILE" \
                    aish capture "$exit_code" "$last_command" 2>/dev/null
            fi
            return $exit_code
        }

        autoload -Uz add-zsh-hook
        add-zsh-hook preexec _aish_preexec
        add-zsh-hook precmd  _aish_precmd

    else
        # bash version: use trap DEBUG and PROMPT_COMMAND implementation
        __aish_capture_on=0

        _aish_preexec() {
            # Skip internal or aish itself
            case "$BASH_COMMAND" in
                _aish_*|aish*|*/aish*) return ;;
            esac
            # Allow per-invocation bypass
            if [ -n "$AISH_CAPTURE_OFF" ]; then
                return
            fi
            # 若屬於需跳過的指令，清空 last_command 並直接返回，避免 postcmd 以舊值誤觸發
            if __aish_should_skip_cmd "$BASH_COMMAND"; then
                : > "$AISH_LAST_CMD_FILE"
                return
            fi
            if [ "$__aish_capture_on" = "1" ]; then
                return
            fi
            : > "$AISH_STDOUT_FILE"
            : > "$AISH_STDERR_FILE"
            local _sanitized
            _sanitized="$(__aish_sanitize_cmd "$BASH_COMMAND")"
            printf "%s" "$_sanitized" > "$AISH_LAST_CMD_FILE"
            exec 4>&1 5>&2
            exec 1> >(tee -a "$AISH_STDOUT_FILE") 2> >(tee -a "$AISH_STDERR_FILE" >&2)
            __aish_capture_on=1
        }

        _aish_postcmd() {
            local exit_code=$?
            local _had_capture=0
            if [ "$__aish_capture_on" = "1" ]; then
                exec 1>&4 4>&- 2>&5 5>&-
                __aish_capture_on=0
                _had_capture=1
            fi
            local last_command
            last_command=$(cat "$AISH_LAST_CMD_FILE" 2>/dev/null)
            # 僅在本次指令確實啟用過捕捉時，才嘗試觸發 aish capture，避免 skip 指令誤觸發
            if [ $_had_capture -eq 1 ] && [ $exit_code -ne 0 ] && [ -n "$last_command" ] && command -v aish >/dev/null 2>&1; then
                __aish_should_trigger "$exit_code" || return $exit_code
                __aish_should_skip_cmd "$last_command" && return
                AISH_STDOUT_FILE="$AISH_STDOUT_FILE" AISH_STDERR_FILE="$AISH_STDERR_FILE" \
                    aish capture "$exit_code" "$last_command" 2>/dev/null
            fi
            return $exit_code
        }

        # Install hook (preserve original PROMPT_COMMAND)
        trap '_aish_preexec' DEBUG
        if [[ $PROMPT_COMMAND != *"_aish_postcmd"* ]]; then
            if [ -z "$PROMPT_COMMAND" ]; then
                PROMPT_COMMAND="_aish_postcmd"
            else
                PROMPT_COMMAND="_aish_postcmd; $PROMPT_COMMAND"
            fi
        fi
    fi

fi

# AISH (AI Shell) Hook - End
