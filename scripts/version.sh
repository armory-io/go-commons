#!/bin/bash

#------------------------------------------------------------------------------------------------------------------
#
# AUTOGENERATED FILE - DO NOT EDIT !
# for details - check https://github.com/armory-io/go-makefile
#
# Calculates the next version based on git tags and current branch.
#
# Examples:
# * Snapshot version: 0.1.0-snapshot.[uncommitted].chore.foo.bar.test.8481993c
# * RC version:       0.1.0-rc.9
# * Release version:  0.1.0
# * master release:   0.1.0-<sha>
#
# Step logic:
# * On release branch, only patch version is stepped depending on the latest git tag matching the branch name
# * On all other branches, if latest tag is not rc, step minor and set patch=0. Otherwise, step rc number
#------------------------------------------------------------------------------------------------------------------

VERSION_TYPE=${VERSION_TYPE:-snapshot}
cp .git/index /tmp/git_index
export GIT_INDEX_FILE=/tmp/git_index
git add --all
LONG_SHA=$(git write-tree)
SHORT_SHA=$(git rev-parse --short "${LONG_SHA}")

[[ ! $VERSION_TYPE =~ snapshot|rc|release|master|main ]] && echo "Usage: $(basename "$0") snapshot|rc|release|master|main [branch-name]" && exit 1

if [[ -z $BRANCH_OVERRIDE ]] ; then BRANCH=$(git rev-parse --abbrev-ref HEAD) ; else BRANCH=$BRANCH_OVERRIDE ; fi

case $BRANCH in
  release-*)
    tmp=$(echo "$BRANCH" | cut -d'-' -f 2)
    read -r br_major br_minor br_patch <<< "${tmp//./ }"
    v=$(git tag --sort=-v:refname | grep "v$br_major.$br_minor" | head -1 | sed 's|v||g') ;;
  *)
    v=$(git tag --sort=-v:refname | head -1 | sed 's|v||g') ;;
esac

read -r major minor patch rc <<< "${v//./ }"
if [[ -z $major && -z $minor ]]; then
    major=$br_major
    minor=$br_minor
    patch=-1
else
    patch=$(echo "$patch" | sed 's|[^0-9]*||g')
fi

# If major.minor.patch exists, reset rc
[[ "x$(git tag -l "v$major.$minor.$patch")" != "x" ]] && rc=""

if [[ -z $major ]]; then
  major=0
  minor=0
  patch=0
fi

case $VERSION_TYPE in
    snapshot)
        if [ "x$(git status --porcelain)" != "x" ] ; then u=".uncommitted"; fi
        br=$(echo ".$BRANCH" | sed 's|[-/_]|.|g')
        if [[ "x$rc" = "x" ]]
        then
            if [[ $BRANCH =~ ^release-* ]] ; then ((patch++)) ; else ((minor++)) && patch=0 ; fi
        fi
        NEW_VERSION="${major}.${minor}.${patch}-snapshot$u$br.$SHORT_SHA"
        ;;
    rc)
        if [[ "x$rc" = "x" ]]
        then
            rc=1
            if [[ $BRANCH =~ ^release-* ]] ; then ((patch++)) ; else ((minor++)) && patch=0 ; fi
        else
            ((rc++))
        fi
        NEW_VERSION="${major}.${minor}.${patch}-rc.${rc}"
        ;;
    release)
        if [[ "x$rc" = "x" ]]
        then
            if [[ $BRANCH =~ ^release-* ]] ; then ((patch++)) ; else ((minor++)) && patch=0 ; fi
        fi
        NEW_VERSION="${major}.${minor}.${patch}"
        ;;
    master)
        if [[ "x$rc" = "x" ]]
        then
            if [[ $BRANCH =~ ^release-* ]] ; then ((patch++)) ; else ((minor++)) && patch=0 ; fi
        fi
        NEW_VERSION="${major}.${minor}.${patch}-$(git rev-parse --short HEAD)"
        ;;
    main)
        if [[ "x$rc" = "x" ]]
        then
            if [[ $BRANCH =~ ^release-* ]] ; then ((patch++)) ; else ((minor++)) && patch=0 ; fi
        fi
        NEW_VERSION="${major}.${minor}.${patch}-$(git rev-parse --short HEAD)"
        ;;
esac

echo -n "$NEW_VERSION"
