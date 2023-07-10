#!usr/bin/env python3

import requests
import subprocess
import sys



def main():
    user = 'ealvar3z'
    local_repo = 'ttv'

    api_url = f'https://api.github.com/repos/{user}/{local_repo}/commits/main'
    headers = {
        'Accept': 'application/vnd.github+json'
    }

    response = requests.get(api_url, headers=headers)
    response_json = response.json()
    latest_commit_sha = response_json['sha']

    # let's truncate the SHA to 8 chars to meet the tag character limit
    tag = latest_commit_sha[:8]


    subprocess.call(['git', 'tag', tag])
    subprocess.call(['git', 'push', 'origin', 'main', '--tags'])

    sys.exit()


if __name__ == "__main__":
    main()
