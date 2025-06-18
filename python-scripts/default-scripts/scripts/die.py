#!../.venv/bin/python

import argparse
import subprocess
import os
import sys
from iterfzf import iterfzf

def get_docker_images():
    result = subprocess.run(
        ["docker", "image", "ls", "--format", "{{.Repository}}:{{.Tag}}"],
        stdout=subprocess.PIPE,
        text=True,
        check=True
    )
    images = result.stdout.strip().split('\n')
    return images

def fzf_select(images):
    result = iterfzf(images)
    return result if result is not None else ""

def main():
    parser = argparse.ArgumentParser(description="Run a Docker image interactively with fzf selection.")
    shell_group = parser.add_mutually_exclusive_group()
    shell_group.add_argument('--bash', action='store_true', help="Use bash as entrypoint")
    shell_group.add_argument('--sh', action='store_true', help="Use sh as entrypoint")
    volume_group = parser.add_mutually_exclusive_group()
    volume_group.add_argument('-v', '--volume', action='store_true', help="Mount ~/docker-mnt/<image_tag> to /mnt/docker-mnt")
    volume_group.add_argument('-vc', '--volume-current', action='store_true', help="Mount current directory to /mnt/docker-mnt")
    parser.add_argument('-i', '--image', type=str, help="Docker image to run (skip fzf selection)")
    args = parser.parse_args()

    entrypoint = "bash" if args.bash or (not args.sh and not args.bash) else "sh"

    if args.image:
        image = args.image
    else:
        images = get_docker_images()
        if not images or images == ['']:
            print("No Docker images found.")
            sys.exit(1)
        image = fzf_select(images)
        if not image:
            print("No image selected.")
            sys.exit(1)

    cmd = [
        "docker", "run", "--rm", "-it",
        "--entrypoint", entrypoint
    ]
    if args.volume_current:
        cwd = os.getcwd()
        cmd += ["-v", f"{cwd}:/mnt/docker-mnt"]
    elif args.volume:
        user_home = os.path.expanduser("~")
        mount_path = os.path.join(user_home, "docker-mnt", image)
        os.makedirs(mount_path, exist_ok=True)
        cmd += ["-v", f"{mount_path}:/mnt/docker-mnt"]
    cmd.append(image)

    # Launch the container
    os.execvp(cmd[0], cmd)

if __name__ == "__main__":
    main()