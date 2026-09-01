# Turn `tiger check` output into the body of the sticky pull-request comment.
#
# Every line tiger check prints is a blocking finding — counted findings
# (escape directives, skipped tests) print only when a package is over its
# tiger.budget.yaml row, as ordinary lines under the TS-D06 line for the
# row — so the report is the run's output verbatim inside a code block,
# followed by the trailer. Slack under a budget is visible in the budget
# file's diff, not here.

/^tiger: [0-9]+ blocking$/ { trailer = $0; next }

{ blocking[++blockingCount] = $0 }

END {
    if (blockingCount > 0) {
        print "```"
        for (i = 1; i <= blockingCount; i++) print blocking[i]
        print "```"
        print ""
    }

    if (trailer != "") print "`" trailer "`"
    if (blockingCount == 0 && trailer == "") {
        print "Clean run: no findings."
    }
}
