# Kubernetes framework

[![Build][build-badge]][build-link]
[![Coverage][coveralls-badge]][coveralls-link]
[![Coverage][coverage-badge]][coverage-link]
[![Quality][quality-badge]][quality-link]
[![Report][report-badge]][report-link]
[![FOSSA][fossa-badge]][fossa-link]
[![License][license-badge]][license-link]
[![Docs][docs-badge]][docs-link]
<!--
[![Libraries][libs-badge]][libs-link]
[![Security][security-badge]][security-link]
-->

[build-badge]: https://github.com/tkrop/go-kube/actions/workflows/build.yaml/badge.svg
[build-link]: https://github.com/tkrop/go-kube/actions/workflows/build.yaml

[coveralls-badge]: https://coveralls.io/repos/github/tkrop/go-kube/badge.svg?branch=main
[coveralls-link]: https://coveralls.io/github/tkrop/go-kube?branch=main

[coverage-badge]: https://app.codacy.com/project/badge/Coverage/cc1c47ec5ce0493caf15c08fa72fc78c
[coverage-link]: https://app.codacy.com/gh/tkrop/go-kube/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_coverage

[quality-badge]: https://app.codacy.com/project/badge/Grade/cc1c47ec5ce0493caf15c08fa72fc78c
[quality-link]: https://app.codacy.com/gh/tkrop/go-kube/dashboard?utm_source=gh&utm_medium=referral&utm_content=&utm_campaign=Badge_grade

[report-badge]: https://goreportcard.com/badge/github.com/tkrop/go-kube
[report-link]: https://goreportcard.com/report/github.com/tkrop/go-kube

[fossa-badge]: https://app.fossa.com/api/projects/git%2Bgithub.com%2Ftkrop%2Ftesting.svg?type=shield&issueType=license
[fossa-link]: https://app.fossa.com/projects/git%2Bgithub.com%2Ftkrop%2Ftesting?ref=badge_shield&issueType=license

[license-badge]: https://img.shields.io/badge/License-MIT-yellow.svg
[license-link]: https://opensource.org/licenses/MIT

[docs-badge]: https://pkg.go.dev/badge/github.com/tkrop/go-kube.svg
[docs-link]: https://pkg.go.dev/github.com/tkrop/go-kube

<!--
[libs-badge]: https://img.shields.io/librariesio/release/github/tkrop/go-kube
[libs-link]: https://libraries.io/github/tkrop/go-kube

[security-badge]: https://snyk.io/test/github/tkrop/go-kube/main/badge.svg
[security-link]: https://snyk.io/test/github/tkrop/go-kube
-->

## Introduction

Goal of `go-kube` Kubernetes framework is to provide some simple patterns for
interacting with Kubernetes API server mostly using the Kubernetes [Go
Client][client-go]. The main pattern provided is a [controller](controller)
framework wrapping a central set of [cache.Indexers][indexers].

[client-go]: <https://github.com/kubernetes/client-go>
[indexers]: <https://pkg.go.dev/k8s.io/client-go/tools/cache#Indexers>


## Building

This project is using a custom build system called [go-make][go-make], that
provides default targets for most common tasks. Makefile rules are generated
based on the project structure and files for common tasks, to initialize,
build, test, and run the components in this repository.

To get started, run one of the following commands.

```bash
make help
make show-targets
```

Read the [go-make manual][go-make-man] for more information about targets
and configuration options.

**Not:** [go-make][go-make] installs `pre-commit` and `commit-msg`
[hooks][git-hooks] calling `make commit` to enforce successful testing and
linting and `make git-verify message` to validate whether the commit message
is following the [conventional commit][convent-commit] best practice.

[go-make]: <https://github.com/tkrop/go-make>
[go-make-man]: <https://github.com/tkrop/go-make/blob/main/MANUAL.md>
[git-hooks]: <https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks>
[convent-commit]: <https://www.conventionalcommits.org/en/v1.0.0/>


## Terms of Usage

This software is open source under the MIT license. You can use it without
restrictions and liabilities. Please give it a star, so that I know. If the
project has more than 25 Stars, I will introduce semantic versions `v1`.


## Contributing

If you like to contribute, please create an issue and/or pull request with a
proper description of your proposal or contribution. I will review it and
provide feedback on it as fast as possible.
