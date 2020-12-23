## Git Workflow
1.  Fork in the cloud
    1.  Visit https://github.com/google/go-flow-levee
    2.  Click `Fork` button (top right) to establish a cloud-based fork.

2.  Clone fork to local storage

    ```bash
    cd $(go env GOPATH)/src/
    git clone https://github.com/<your-github-username>/go-flow-levee.git

    cd go-flow-levee
    git remote add upstream https://github.com/google/go-flow-levee.git

    # Never push to upstream master
    git remote set-url --push upstream no_push

    # Confirm that your remotes make sense:
    git remote -v
    ```

3.  Branch

    Get your local master up to date:

    ```bash
    cd $(go env GOPATH)/src/go-flow-levee
    git fetch upstream
    git checkout master
    git merge upstream/master
    ```

    Branch from it:
    ```bash
    git checkout -b featurebranch
    ```

4.  Keep your branch in sync
    ```bash
    git fetch upstream
    git merge upstream/master
    ```

5.  Pushing your commits
    ```bash
    git push origin featurebranch
    ```

    **Note:** Please avoid force-pushing, as it can break links to commits and cause GitHub to lose track of comment threads.

6.  Create a Pull Request
    1.  Go to https://github.com/google/go-flow-levee
    2.  Click on the "Compare & pull request" button.

    **Note:** When merging your PR, please use a squash merge.
