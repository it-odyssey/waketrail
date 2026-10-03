#!/usr/bin/env bash

__waketrail_begin_capture() {
    WAKETRAIL_STDOUT_FILE="$(mktemp "${TMPDIR:-/tmp}/waketrail-stdout.XXXXXX")"
    WAKETRAIL_STDERR_FILE="$(mktemp "${TMPDIR:-/tmp}/waketrail-stderr.XXXXXX")"

    exec {WAKETRAIL_SAVED_STDOUT}>&1
    exec {WAKETRAIL_SAVED_STDERR}>&2

    exec > >(tee "$WAKETRAIL_STDOUT_FILE" >&${WAKETRAIL_SAVED_STDOUT})
    WAKETRAIL_STDOUT_TEE_PID=$!

    exec 2> >(tee "$WAKETRAIL_STDERR_FILE" >&${WAKETRAIL_SAVED_STDERR})
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

__waketrail_preexec() {
    local cmd="$BASH_COMMAND"

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

    WAKETRAIL_LAST_COMMAND="$cmd"

    WAKETRAIL_CAPTURE_MODE="$(waketrail classify "$cmd" 2>/dev/null)"

    if [[ -z "$WAKETRAIL_CAPTURE_MODE" ]]; then
        WAKETRAIL_CAPTURE_MODE="none"
    fi

    WAKETRAIL_COMMAND_STARTED_AT="$(date +%s%N)"

    if [[ "$WAKETRAIL_CAPTURE_MODE" == "output" ||
          "$WAKETRAIL_CAPTURE_MODE" == "bounded" ]]; then
        __waketrail_begin_capture
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

        waketrail record \
            "${record_args[@]}" \
            "$WAKETRAIL_LAST_COMMAND" \
            >/dev/null 2>&1

        if [[ -n "${WAKETRAIL_STDOUT_FILE:-}" ]]; then
            rm -f "$WAKETRAIL_STDOUT_FILE"
        fi

        if [[ -n "${WAKETRAIL_STDERR_FILE:-}" ]]; then
            rm -f "$WAKETRAIL_STDERR_FILE"
        fi

        unset WAKETRAIL_LAST_COMMAND
        unset WAKETRAIL_COMMAND_STARTED_AT
        unset WAKETRAIL_CAPTURE_MODE
        unset WAKETRAIL_STDOUT_FILE
        unset WAKETRAIL_STDERR_FILE
    fi
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

trap '__waketrail_preexec' DEBUG

__waketrail_register_precmd

unset WAKETRAIL_LAST_COMMAND
unset WAKETRAIL_COMMAND_STARTED_AT
unset WAKETRAIL_CAPTURE_MODE
unset WAKETRAIL_STDOUT_FILE
unset WAKETRAIL_STDERR_FILE