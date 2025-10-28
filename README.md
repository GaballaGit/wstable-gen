This is a script to update acmcsuf's workshops page tables.

# How to use it
---
- First clone the repo and cd into it.

- Then in a dot env file type the absolute file path into your acmcsuf.com repo as so:
`ABS_PATH="/Users/gaballa/Documents/ACM/acmcsuf/remote/acmcsuf.com/"`

- Then just use the command
`go run wsLinksParser.go`

- This will write to the tables file that workshops in your acmcsuf.com repo uses.

---
### Note: You will still need to run npm run all in the acmcsuf repo since this does not pass the fmt check
