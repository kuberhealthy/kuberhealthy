# Dagger Build

Running workflows outside of the source repo isn't possible, due to org policy I think. I started updating the workflows only to not be able to run them, created a simple dagger ci pipeline so that it can be run locally and run by CI outside of GH.

Had a small window, which ended up being smaller then anticipated. Following is an example command:

`dagger call build --dir="../" --repo="krisjohnstone" --function="deployment-check" --version="v1.20.0"`
