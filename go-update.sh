# When doing updates you can force pull a specific version, without
# waiting for the indexing, like:
# `go get -u github.com/paulfdunn/go-helper/logh@v1.0.7`
find  . -type d -not -path "./.git*" | xargs -I % -L1 sh -c 'cd %; pwd; go get -u; cd ..'
find  . -type d -not -path "./.git*" | xargs -I % -L1 sh -c 'cd %; pwd; go mod tidy; cd ..'