## Cerulean Owl

#### Overview

A Linkedin dorker that uses Google's customsearch API to generate a list of names related to a company.

#### Usage

You'll need an API key an CX value from registering for Google's customsearch API (it's free, but limited to 100 queries per day TMK).

Install Cerulean Own: `go install github.com/redskal/cerulean-owl@latest`

Run it once to generate your configuration file. It will be located under your user's home directory in a directory named CeruleanOwl (prefixed with a dot for Linux users). Adjust `settings.yaml` to include your customsearch keys.

Run the program:
`./cerulean-owl -target "Microsoft"` - It should dump a list of names of employees currently working at Microsoft.
