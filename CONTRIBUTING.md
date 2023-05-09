# NVIDIA IPAM Plugin

## How to Contribute

NVIDIA IPAM plugin is [Apache 2.0 licensed](LICENSE) and accepts contributions via GitHub pull requests.
This document outlines some of the conventions on development workflow, commit message formatting,
contact points and other resources to make it easier to get your contribution accepted.

## Coding Style

Please follows the standard formatting recommendations and language idioms set out in [Effective Go](https://golang.org/doc/effective_go.html) and in the [Go Code Review Comments wiki](https://github.com/golang/go/wiki/CodeReviewComments).

## Format of the patch

Each patch is expected to comply with the following format:

```text
Change summary

More detailed explanation of your changes: Why and how.
Wrap it to 72 characters.
See [here] (http://chris.beams.io/posts/git-commit/)
for some more good advices.

[Fixes #NUMBER (or URL to the issue)]
```

For example:

```text
Fix poorly named identifiers
  
One identifier, fnname, in func.go was poorly named.  It has been renamed
to fnName.  Another identifier retval was not needed and has been removed
entirely.

Fixes #1
```

## Contributing Code

* Make sure to create an [Issue](https://github.com/Mellanox/nvidia-k8s-ipam/issues) for bug fix or the feature request.
* **For bugs**: For the bug fixes, please follow the issue template format while creating a issue.  If you have already found a fix, feel free to submit a Pull Request referencing the Issue you created. Include the `Fixes #` syntax to link it to the issue you're addressing.
* **For feature requests**, For the feature requests, please follow the issue template format while creating a feature requests.
  * Please make sure each PR are compiling and CI checks are passing.
  * In order to ensure your PR can be reviewed in a timely manner, please keep PRs small.

Once you're ready to contribute code back to this repo, start with these steps:

* Fork the project.
* Clone the fork to your machine:

```shell
git clone https://github.com/Mellanox/nvidia-k8s-ipam.git
```

* Create a topic branch with prefix `dev/` for your change and checkout that branch:

```shell
git checkout -b dev/some-topic-branch
```

* Make your changes to the code and add tests to cover contributed code.
* Run `make build && make test` to validate it builds and will not break current functionality.
* Commit your changes and push them to your fork.
* Open a pull request for the appropriate project.
* Maintainers will review your pull request, suggest changes, run tests and eventually merge or close the request.
