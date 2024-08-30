# Update Golden Tests

Instead of manually updating the golden tests, once you've verified the output of the tests is correct, then you
can simply update all the `expected output files` files by running

```bash
go test ./internal/clientgen -golden-update
```
