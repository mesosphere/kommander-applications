# kommander-applications

This repo is dedicated to storing the HelmRelease and other info needed for Kommander's Applications.

### Pre Commit

This repo uses https://pre-commit.com/ to run pre-commit hooks. Please install pre-commit and run `pre-commit install` in the root directory before committing.

### Running Tests

You can run tests with `make go-test`. If your tests do not meet a certain coverage threshold, your build will fail.
