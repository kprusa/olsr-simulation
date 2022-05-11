echo "Checking for Go"
FILE="./go/"
if ! test -e "$FILE"; then
  echo "Downloading Go..."
  curl https://dl.google.com/go/go1.18.2.linux-amd64.tar.gz --output go1.18.2.linux-amd64.tar.gz
  echo "Done downloading Go."

  echo "Extracting Go..."
  echo "This could take a few minutes"
  tar -xzf go1.18.2.linux-amd64.tar.gz
  echo "Done extracting Go."
fi

echo "Building network simulation..."
GOROOT=./go
PATH=./go/bin:$PATH
if ./go/bin/go build; then
  echo "Done building network simulation."
else
  echo "Could not build network simulation."
fi
echo "Done."
