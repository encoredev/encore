# Update Golden Tests

Instead of manually updating the golden tests, once you've verified the output of the tests is correct, then you
can simply update all the `.golden` files by running

```bash
 go test ./compiler/internal/codegen -golden-update
```
