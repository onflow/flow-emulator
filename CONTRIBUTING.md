# Contributing to the Flow Emulator

The following is a set of guidelines for contributing to the Flow Emulator.
These are mostly guidelines, not rules.
Use your best judgment, and feel free to propose changes to this document in a pull request.

## Table Of Contents

[Getting Started](#project-overview)

[How Can I Contribute?](#how-can-i-contribute)

- [Reporting Bugs](#reporting-bugs)
- [Suggesting Enhancements](#suggesting-enhancements)
- [Your First Code Contribution](#your-first-code-contribution)
- [Pull Requests](#pull-requests)

[Styleguides](#styleguides)

- [Git Commit Messages](#git-commit-messages)
- [Go Styleguide](#go-styleguide)

[Additional Notes](#additional-notes)

## Development

Run a development version of emulator server:

```shell script
make run
```

Run all unit tests in this repository:

```shell script
make test
```

## Building

To build the container locally, use `make docker-build` from the root of this repository.

Images are automatically built and pushed to `gcr.io/flow-container-registry/emulator` via [Team City](https://ci.eng.dapperlabs.com/project/Flow_FlowGo_FlowEmulator) on any push to master, i.e. Pull Request merge

## Deployment

The emulator is currently being deployed via [Team City](https://ci.eng.dapperlabs.com/project/Flow_FlowGo_FlowEmulator)
All commands relating to the deployment process live in the `Makefile` in this directory

The deployment has a persistent volume and should keep state persistent.
The deployment file for the Ingress/Service/Deployment combo, and the
Persistent Volume Claim are located in the k8s folder.

#### From a Kubernetes Pod

If you are in the same namespace (flow), you can simply access the service via it's service name, e.g. `flow-emulator-v1`.
If you are in a different namespace, you will need to use a [Service without selectors](https://kubernetes.io/docs/concepts/services-networking/service/#services-without-selectors)
e.g.

```yaml
kind: Service
apiVersion: v1
metadata:
  name: flow-emulator-v1
  namespace: YOUR_NAMESPACE
spec:
  type: ExternalName
  externalName: flow-emulator-v1.flow.svc.cluster.local
  ports:
      protocol: TCP
      port: 3569
      targetPort: 3569
```

### Creating your own deployment

Our current deployment settings are available in the [k8s](cmd/emulator/k8s) sub directory, if you'd like to setup the emulator in your own Kubernetes cluster. We are using the `Traefik` Ingress controller, but if that is not needed for your purposes, that can be removed, along with any corresponding annotations in the deployment file.

If not using Kubernetes, you can run the Docker container independently. Make sure to run the Docker container with the gRPC port exposed (default is `3569`). Metrics are also available on port `8080` on the `/metrics` endpoint.

To gain persistence for data on the emulator, you will have to provision a volume for the docker container. We've done this through `Persistent Volumes` on Kubernetes, but a mounted volume would suffice. The mount point can be set with the `FLOW_DBPATH` environment variable. We suggest a volume of at least 10GB (100GB for a long-term deployment).

Make sure the emulator also has access to the same `flow.json` file, or always launch it with the same service key, as mentioned above.

```bash
docker run -e FLOW_SERVICEPUBLICKEY=<hex-encoded key> -e FLOW_DBPATH="/flowdb" -v "$(pwd)/flowdb":"/flowdb"  -p 3569:3569 gcr.io/flow-container-registry/emulator
```


## How Can I Contribute?

### Reporting Bugs

#### Before Submitting A Bug Report

- **Search existing issues** to see if the problem has already been reported.
  If it has **and the issue is still open**, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Bug Report?

Explain the problem and include additional details to help maintainers reproduce the problem:

- **Use a clear and descriptive title** for the issue to identify the problem.
- **Describe the exact steps which reproduce the problem** in as many details as possible.
  When listing steps, **don't just say what you did, but explain how you did it**.
- **Provide specific examples to demonstrate the steps**.
  Include links to files or GitHub projects, or copy/pasteable snippets, which you use in those examples.
  If you're providing snippets in the issue,
  use [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines).
- **Describe the behavior you observed after following the steps** and point out what exactly is the problem with that behavior.
- **Explain which behavior you expected to see instead and why.**
- **Include error messages and stack traces** which show the output / crash and clearly demonstrate the problem.

Provide more context by answering these questions:

- **Can you reliably reproduce the issue?** If not, provide details about how often the problem happens
  and under which conditions it normally happens.

Include details about your configuration and environment:

- **What is the version of the emulator you're using**?
- **What's the name and version of the Operating System you're using**?

### Suggesting Enhancements

#### Before Submitting An Enhancement Suggestion

- **Perform a cursory search** to see if the enhancement has already been suggested.
  If it has, add a comment to the existing issue instead of opening a new one.

#### How Do I Submit A (Good) Enhancement Suggestion?

Enhancement suggestions are tracked as [GitHub issues](https://guides.github.com/features/issues/).
Create an issue and provide the following information:

- **Use a clear and descriptive title** for the issue to identify the suggestion.
- **Provide a step-by-step description of the suggested enhancement** in as many details as possible.
- **Provide specific examples to demonstrate the steps**.
  Include copy/pasteable snippets which you use in those examples,
  as [Markdown code blocks](https://help.github.com/articles/markdown-basics/#multiple-lines).
- **Describe the current behavior** and **explain which behavior you expected to see instead** and why.
- **Explain why this enhancement would be useful** to emulator users.

### Your First Code Contribution

Unsure where to begin contributing to the Flow Emulator?
You can start by looking through these `good-first-issue` and `help-wanted` issues:

- [Good first issues](https://github.com/onflow/flow-emulator/labels/good%20first%20issue):
  issues which should only require a few lines of code, and a test or two.
- [Help wanted issues](https://github.com/onflow/flow-emulator/labels/help%20wanted):
  issues which should be a bit more involved than `good-first-issue` issues.

Both issue lists are sorted by total number of comments.
While not perfect, number of comments is a reasonable proxy for impact a given change will have.

### Pull Requests

The process described here has several goals:

- Maintain code quality
- Fix problems that are important to users
- Engage the community in working toward the best possible UX
- Enable a sustainable system for the emulator's maintainers to review contributions

Please follow the [styleguides](#styleguides) to have your contribution considered by the maintainers.
Reviewer(s) may ask you to complete additional design work, tests,
or other changes before your pull request can be ultimately accepted.

## Styleguides

Before contributing, make sure to examine the project to get familiar with the patterns and style already being used.

### Git Commit Messages

- Use the present tense ("Add feature" not "Added feature")
- Use the imperative mood ("Move cursor to..." not "Moves cursor to...")
- Limit the first line to 72 characters or less
- Reference issues and pull requests liberally after the first line

### Go Styleguide

The majority of this project is written Go.

We try to follow the coding guidelines from the Go community.

- Code should be formatted using `gofmt`
- Code should pass the linter: `make lint`
- Code should follow the guidelines covered in
  [Effective Go](http://golang.org/doc/effective_go.html)
  and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Code should be commented
- Code should pass all tests: `make test`

## Additional Notes

Thank you for your interest in contributing to the Flow Emulator!
