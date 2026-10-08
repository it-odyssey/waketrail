#!/usr/bin/env bash

# One private directory owns this command's spools and optional Git snapshot.
# A usable per-user runtime directory wins; mktemp keeps the fallback private.
__waketrail_prepare_tmp() {
    if [[ -n "${WAKETRAIL_TMP_DIR:-}" ]]; then
        return 0
    fi
    local base="${TMPDIR:-/tmp}"
    if [[ -n "${XDG_RUNTIME_DIR:-}" && -d "$XDG_RUNTIME_DIR" &&
          -O "$XDG_RUNTIME_DIR" && -w "$XDG_RUNTIME_DIR" &&
          "$(stat -c %a -- "$XDG_RUNTIME_DIR" 2>/dev/null)" == "700" ]]; then
        base="$XDG_RUNTIME_DIR"
    fi
    WAKETRAIL_TMP_DIR="$(mktemp -d "$base/waketrail.XXXXXX" 2>/dev/null)" || return 1
}

__waketrail_cleanup_files() {
    if [[ -n "${WAKETRAIL_TMP_DIR:-}" ]]; then
        rm -f -- "$WAKETRAIL_TMP_DIR/stdout" "$WAKETRAIL_TMP_DIR/stderr" "$WAKETRAIL_TMP_DIR/git"
        rmdir -- "$WAKETRAIL_TMP_DIR" 2>/dev/null || true
    fi
    unset WAKETRAIL_TMP_DIR WAKETRAIL_STDOUT_FILE WAKETRAIL_STDERR_FILE WAKETRAIL_GIT_BEFORE_FILE
}

__waketrail_begin_capture() {
    __waketrail_prepare_tmp || return 1
    WAKETRAIL_STDOUT_FILE="$WAKETRAIL_TMP_DIR/stdout"
    WAKETRAIL_STDERR_FILE="$WAKETRAIL_TMP_DIR/stderr"

    exec {WAKETRAIL_SAVED_STDOUT}>&1
    exec {WAKETRAIL_SAVED_STDERR}>&2

    exec > >(waketrail capture-stream --output "$WAKETRAIL_STDOUT_FILE" >&${WAKETRAIL_SAVED_STDOUT} 2>/dev/null)
    WAKETRAIL_STDOUT_TEE_PID=$!

    exec 2> >(waketrail capture-stream --output "$WAKETRAIL_STDERR_FILE" >&${WAKETRAIL_SAVED_STDERR} 2>/dev/null)
    WAKETRAIL_STDERR_TEE_PID=$!
}

__waketrail_end_capture() {
    if [[ -z "${WAKETRAIL_SAVED_STDOUT:-}" ]]; then
        return
    fi

    exec 1>&${WAKETRAIL_SAVED_STDOUT}
    exec 2>&${WAKETRAIL_SAVED_STDERR}

    exec {WAKETRAIL_SAVED_STDOUT}>&-
    exec {WAKETRAIL_SAVED_STDERR}>&-

    if [[ -n "${WAKETRAIL_STDOUT_TEE_PID:-}" ]]; then
        wait "$WAKETRAIL_STDOUT_TEE_PID" 2>/dev/null || true
    fi

    if [[ -n "${WAKETRAIL_STDERR_TEE_PID:-}" ]]; then
        wait "$WAKETRAIL_STDERR_TEE_PID" 2>/dev/null || true
    fi

    unset WAKETRAIL_SAVED_STDOUT
    unset WAKETRAIL_SAVED_STDERR
    unset WAKETRAIL_STDOUT_TEE_PID
    unset WAKETRAIL_STDERR_TEE_PID
}

# DEBUG exposes pipeline/list fragments. A fresh history entry lets the existing
# classifier inspect the entire submitted line. Ignored/disabled history is not
# proof of a simple command, so those commands retain metadata only.
__waketrail_read_history() {
    local entry
    WAKETRAIL_HISTORY_CURRENT=0
    WAKETRAIL_HISTORY_COMMAND=""
    entry="$(HISTTIMEFORMAT= builtin history 1 2>/dev/null)"
    if [[ "$entry" =~ ^[[:space:]]*([0-9]+)[[:space:]]+(.*)$ ]]; then
        WAKETRAIL_HISTORY_CURRENT="${BASH_REMATCH[1]}"
        WAKETRAIL_HISTORY_COMMAND="${BASH_REMATCH[2]}"
    fi
}

__waketrail_preexec() {
    local cmd="$BASH_COMMAND"
    local full_command_known=0

    case "$cmd" in
        __waketrail_*|\
        starship_precmd|\
        starship_preexec|\
        __starship_*|\
        __vsc_*|\
        __zoxide_*|\
        printf\ "\\033]"*|\
        trap*|\
        PROMPT_COMMAND=*|\
        unset\ WAKETRAIL_*)
            return
            ;;
    esac

    __waketrail_read_history
    if [[ -n "$WAKETRAIL_HISTORY_COMMAND" &&
          "$WAKETRAIL_HISTORY_CURRENT" != "${WAKETRAIL_HISTORY_PREVIOUS:-0}" ]]; then
        cmd="$WAKETRAIL_HISTORY_COMMAND"
        full_command_known=1
    fi

    # A second DEBUG event can occur before the next prompt (command lists).
    # Restore the previous descriptors before replacing its capture handles.
    # Full compound-command attribution remains a separate compatibility review.
    if [[ -n "${WAKETRAIL_LAST_COMMAND:-}" ]]; then
        __waketrail_end_capture
        __waketrail_cleanup_files
    fi
    WAKETRAIL_LAST_COMMAND="$cmd"

    WAKETRAIL_CAPTURE_MODE="none"
    if [[ "$full_command_known" == 1 ]]; then
        WAKETRAIL_CAPTURE_MODE="$(waketrail classify "$cmd" 2>/dev/null)"
    fi

    if [[ -z "$WAKETRAIL_CAPTURE_MODE" ]]; then
        WAKETRAIL_CAPTURE_MODE="none"
    fi

    # Only snapshot Git state for commands that may change it. The recorder
    # independently checks the command and will ignore a missing snapshot.
    case "$cmd" in
        git\ add\ *|git\ commit\ *|git\ switch\ *|git\ checkout\ *|git\ merge\ *|git\ rebase\ *|git\ reset\ *|git\ restore\ *|git\ rm\ *|git\ mv\ *|git\ stash*|git\ cherry-pick\ *|git\ revert\ *|git\ pull*)
            if __waketrail_prepare_tmp; then
                WAKETRAIL_GIT_BEFORE_FILE="$WAKETRAIL_TMP_DIR/git"
                waketrail git-snapshot --cwd "$PWD" --output "$WAKETRAIL_GIT_BEFORE_FILE" >/dev/null 2>&1 || {
                    rm -f -- "$WAKETRAIL_GIT_BEFORE_FILE"
                    unset WAKETRAIL_GIT_BEFORE_FILE
                }
            fi
            ;;
    esac

    WAKETRAIL_COMMAND_STARTED_AT="$(date +%s%N)"

    if [[ "$WAKETRAIL_CAPTURE_MODE" == "output" ||
          "$WAKETRAIL_CAPTURE_MODE" == "bounded" ]]; then
        __waketrail_begin_capture || {
            # A temp-directory failure degrades to metadata, without redirecting
            # the command into missing capture files.
            __waketrail_cleanup_files
            WAKETRAIL_CAPTURE_MODE="none"
        }
    fi
}

__waketrail_precmd() {
    local exit_code=$?

    __waketrail_end_capture

    if [[ -n "${WAKETRAIL_LAST_COMMAND:-}" ]]; then
        local ended_at
        ended_at="$(date +%s%N)"

        local record_args=(
            --cwd "$PWD"
            --exit-code "$exit_code"
            --capture-mode "$WAKETRAIL_CAPTURE_MODE"
            --started-at "$WAKETRAIL_COMMAND_STARTED_AT"
            --ended-at "$ended_at"
        )

        if [[ "$WAKETRAIL_CAPTURE_MODE" == "output" ||
              "$WAKETRAIL_CAPTURE_MODE" == "bounded" ]]; then
            if [[ -n "${WAKETRAIL_STDOUT_FILE:-}" ]]; then
                record_args+=(
                    --stdout-file "$WAKETRAIL_STDOUT_FILE"
                )
            fi

            if [[ -n "${WAKETRAIL_STDERR_FILE:-}" ]]; then
                record_args+=(
                    --stderr-file "$WAKETRAIL_STDERR_FILE"
                )
            fi
        fi

        if [[ -n "${WAKETRAIL_GIT_BEFORE_FILE:-}" ]]; then
            record_args+=(--git-before-file "$WAKETRAIL_GIT_BEFORE_FILE")
        fi

        waketrail record \
            "${record_args[@]}" \
            "$WAKETRAIL_LAST_COMMAND" \
            >/dev/null 2>&1

        __waketrail_cleanup_files
        unset WAKETRAIL_LAST_COMMAND
        unset WAKETRAIL_COMMAND_STARTED_AT
        unset WAKETRAIL_CAPTURE_MODE
        unset WAKETRAIL_STDOUT_FILE
        unset WAKETRAIL_STDERR_FILE
    fi
    __waketrail_read_history
    WAKETRAIL_HISTORY_PREVIOUS="$WAKETRAIL_HISTORY_CURRENT"
}

__waketrail_register_precmd() {
    local declaration

    declaration="$(declare -p PROMPT_COMMAND 2>/dev/null || true)"

    if [[ "$declaration" == "declare -a"* ]]; then
        local entry

        for entry in "${PROMPT_COMMAND[@]}"; do
            if [[ "$entry" == "__waketrail_precmd" ]]; then
                return
            fi
        done

        PROMPT_COMMAND=(
            "__waketrail_precmd"
            "${PROMPT_COMMAND[@]}"
        )

        return
    fi

    if [[ "${PROMPT_COMMAND:-}" == *"__waketrail_precmd"* ]]; then
        return
    fi

    if [[ -n "${PROMPT_COMMAND:-}" ]]; then
        PROMPT_COMMAND="__waketrail_precmd;${PROMPT_COMMAND}"
    else
        PROMPT_COMMAND="__waketrail_precmd"
    fi
}

__waketrail_register_debug_trap() {
    local existing_trap
    local existing_command=""

    existing_trap="$(trap -p DEBUG)"

    if [[ -n "$existing_trap" &&
          "$existing_trap" != *"__waketrail_preexec"* ]]; then
        existing_command="${existing_trap#trap -- \'}"
        existing_command="${existing_command%\' DEBUG}"
    fi

    if [[ -n "$existing_command" ]]; then
        trap "__waketrail_preexec; $existing_command" DEBUG
    else
        trap '__waketrail_preexec' DEBUG
    fi
}

__waketrail_return_status() { return "$1"; }

__waketrail_exit_cleanup() {
    local exit_code="$1"
    trap - DEBUG
    __waketrail_end_capture
    __waketrail_cleanup_files
    if [[ -n "${WAKETRAIL_PREVIOUS_EXIT_TRAP:-}" ]]; then
        # Both branches preserve $? for a pre-existing EXIT handler, including
        # with errexit enabled. Its explicit exit, if any, still takes effect.
        if __waketrail_return_status "$exit_code"; then
            eval -- "$WAKETRAIL_PREVIOUS_EXIT_TRAP"
        else
            eval -- "$WAKETRAIL_PREVIOUS_EXIT_TRAP"
        fi
    fi
    return "$exit_code"
}

__waketrail_register_exit_trap() {
    local existing_trap quoted_command
    existing_trap="$(trap -p EXIT)"
    if [[ "$existing_trap" == *"__waketrail_exit_cleanup"* ]]; then
        return
    fi
    WAKETRAIL_PREVIOUS_EXIT_TRAP=""
    if [[ -n "$existing_trap" ]]; then
        quoted_command="${existing_trap#trap -- }"
        quoted_command="${quoted_command% EXIT}"
        eval "WAKETRAIL_PREVIOUS_EXIT_TRAP=$quoted_command"
    fi
    trap '__waketrail_exit_cleanup "$?"' EXIT
}

__waketrail_read_history
WAKETRAIL_HISTORY_PREVIOUS="$WAKETRAIL_HISTORY_CURRENT"
__waketrail_register_debug_trap
__waketrail_register_precmd
__waketrail_register_exit_trap

unset WAKETRAIL_LAST_COMMAND
unset WAKETRAIL_COMMAND_STARTED_AT
unset WAKETRAIL_CAPTURE_MODE
unset WAKETRAIL_STDOUT_FILE
unset WAKETRAIL_STDERR_FILE
