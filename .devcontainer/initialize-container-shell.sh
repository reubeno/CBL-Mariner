#
# This script is run by the shell in the devcontainer.
#

# Aliases for convenience
alias make='make -j$(nproc) --no-print-directory'
alias sudo='sudo --preserve-env="CHROOT_DIR"'

# Set default working dir to where the Makefile is.
cd /sources/toolkit || true

# Display some help.
cat $HOME/welcome.txt
