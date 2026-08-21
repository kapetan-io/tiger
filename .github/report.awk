# Turn `tiger check` output into the body of the sticky pull-request comment.
#
# Blocking findings print verbatim — they fail the run and every one needs
# reading. Advisories never fail the run, so they collapse to a per-rule count
# with up to three example positions, and the full list moves into a collapsed
# block. A wall of advisories that scrolls the blocking result off the screen
# teaches reviewers to skip the comment, which costs more than the rule that
# printed them.

/^tiger: [0-9]+ blocking, [0-9]+ advisory$/ { trailer = $0; next }

match($0, / TS-[A-Z]+[0-9]+ \[advisory\]:/) {
    rule = substr($0, RSTART + 1, RLENGTH - 13)
    if (!(rule in count)) { rules[++ruleCount] = rule }
    count[rule]++
    advisory[++advisoryCount] = $0
    if (count[rule] <= 3) {
        position = $0
        sub(/: TS-.*$/, "", position)
        if (examples[rule] == "") examples[rule] = position
        else examples[rule] = examples[rule] "<br>" position
    }
    next
}

{ blocking[++blockingCount] = $0 }

END {
    if (blockingCount > 0) {
        print "```"
        for (i = 1; i <= blockingCount; i++) print blocking[i]
        print "```"
        print ""
    }

    if (advisoryCount > 0) {
        noun = "advisories"
        if (advisoryCount == 1) noun = "advisory"
        printf "**%d %s** — these never fail the run.\n\n", advisoryCount, noun
        print "| rule | count | where |"
        print "|---|---:|---|"
        # Descending by count, so whatever dominates the channel is named first.
        for (i = 1; i <= ruleCount; i++) {
            best = 0
            for (j = 1; j <= ruleCount; j++) {
                if (!done[j] && (best == 0 || count[rules[j]] > count[rules[best]])) best = j
            }
            done[best] = 1
            rule = rules[best]
            printf "| `%s` | %d | %s |\n", rule, count[rule], examples[rule]
        }
        print ""
        printf "<details><summary>all %d advisory lines</summary>\n\n", advisoryCount
        print "```"
        for (i = 1; i <= advisoryCount; i++) print advisory[i]
        print "```"
        print ""
        print "</details>"
        print ""
    }

    if (trailer != "") print "`" trailer "`"
    if (blockingCount == 0 && advisoryCount == 0 && trailer == "") {
        print "Clean run: no findings, no advisories."
    }
}
