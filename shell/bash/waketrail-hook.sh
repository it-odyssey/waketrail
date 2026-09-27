#!/usr/bin/env bash

__waketrail_preexec() {
    local cmd="$BASH_COMMAND"

    case "$cmd" in
        __waketrail_*|__vsc_*|__zoxide_*|printf\ "\\033]*"|trap*|PROMPT_COMMAND=*|unset\ WAKETRAIL_*)
            return
            ;;
    esac

    WAKETRAIL_LAST_COMMAND="$cmd"
    WAKETRAIL_COMMAND_STARTED_AT="$(date +%s%N)"
}

__waketrail_precmd() {
    local exit_code=$?

    if [[ -n "${WAKETRAIL_LAST_COMMAND:-}" ]]; then
        local ended_at
        ended_at="$(date +%s%N)"

        waketrail record \
            --cwd "$PWD" \
            --exit-code "$exit_code" \
            --started-at "$WAKETRAIL_COMMAND_STARTED_AT" \
            --ended-at "$ended_at" \
            "$WAKETRAIL_LAST_COMMAND" \
            >/dev/null 2>&1

        unset WAKETRAIL_LAST_COMMAND
        unset WAKETRAIL_COMMAND_STARTED_AT
    fi
}

trap '__waketrail_preexec' DEBUG

if [[ "${PROMPT_COMMAND:-}" != *"__waketrail_precmd"* ]]; then
    if [[ -n "${PROMPT_COMMAND:-}" ]]; then
        PROMPT_COMMAND="__waketrail_precmd;${PROMPT_COMMAND}"
    else
        PROMPT_COMMAND="__waketrail_precmd"
    fi
fi

unset WAKETRAIL_LAST_COMMAND
unset WAKETRAIL_COMMAND_STARTED_AT