#!/usr/bin/env bash

# This script requires bash shell - sorry.

NEW_TAG="${1}"

set -ue

if test "${NEW_TAG}" = "" ; then
	echo "!! Usage: $0 <release tag>" >&2
	exit 1
fi

# Target (version) tag must match version scheme vMAJOR.MINOR.PATCH with an
# optional pre-release tag.
if ! echo "${NEW_TAG}" | grep -E -q '^v[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)*$'; then
	echo "!! Malformed version tag: '${NEW_TAG}', must match 'vMAJOR.MINOR.PATCH(-rcX)'" >&2
	exit 1
fi

VERSION="${NEW_TAG/#v/}"
echo "VERSION=$VERSION"

# Check whether we are in correct branch of local repository
RELEASE_BRANCH="release-${VERSION/%-*}"

currentBranch=$(git branch --show-current)
if test "$currentBranch" != "${RELEASE_BRANCH}"; then
	echo "!! Please checkout branch '${RELEASE_BRANCH}' (currently in branch: '${currentBranch}')" >&2
	exit 1
fi

echo ">> Working in release branch '${RELEASE_BRANCH}'"

echo ">> Updating VERSION"
echo "$VERSION" > VERSION

echo ">> Updating manifests"
make manifests-local IMAGE_TAG="$NEW_TAG"

echo ">> Updating helm Chart"
sed -i -e "s/^appVersion: .*\$/appVersion: ${NEW_TAG}/g" -e "s/^version: .*\$/version: ${NEW_TAG}/g" ./helm/Chart.yaml
make update-helm-docs-local





