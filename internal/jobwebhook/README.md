# jobwebhook

The jobwebhook package implements a conversion webhook for legacy `KuberhealthyJob` resources. It decodes incoming conversion requests, translates each job into a v2 `KuberhealthyCheck`, and returns the converted objects to the API server.

Its responsibilities are limited to job-to-check conversion and HTTP handling for that conversion endpoint. Admission logic and other API interactions live in separate packages.
