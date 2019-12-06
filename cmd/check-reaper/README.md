

## Checker pod reaper

This container retains checker pods that have run or failed according to the following order of rules:

- If a checker pod in any namespace has a label with the key "kh-check-name"

- If the checker pod is older than 1 hour

- If there are more than 5 checker pods

- If there are any failed checker pods, keep all 5 checker pods around

- If all 5 checker pods have passed, retain the latest 2 checker pods


