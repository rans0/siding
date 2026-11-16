# tdd

1. Write one failing test for the next behavior, through the public interface (what a caller sees, not internals). Run it; confirm it fails for the right reason. Never write tests for later behaviors ahead.
2. Write the minimum code to make it pass. Run it; confirm green.
3. Refactor only while green.
4. Record in `testing.md`: test command, what it covers, result (paste the summary line).

Bug fix: the first test reproduces the bug (diagnose skill).
