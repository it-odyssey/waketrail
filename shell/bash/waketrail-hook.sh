#!/usr/bin/env bash

__waketrail_preexec() {
    local cmd="$BASH_COMMAND"

    # Ignore WakeTrail internals and shell/prompt framework commands.
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

__waketrail_register_precmd() {
    local declaration

    declaration="$(declare -p PROMPT_COMMAND 2>/dev/null || true)"

    # Modern Bash allows PROMPT_COMMAND to be an array.
    # Preserve every existing prompt hook and add WakeTrail exactly once.
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

    # Fall back to string behavior for shells/configurations that use
    # the traditional PROMPT_COMMAND string.
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