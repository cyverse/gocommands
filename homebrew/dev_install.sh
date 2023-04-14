# As you are developing, you’ll also need to set HOMEBREW_NO_INSTALL_FROM_API=1 
# before any install, reinstall or upgrade commands, to force brew to use 
# the local repository instead of the API.
HOMEBREW_NO_INSTALL_FROM_API=1
HOMEBREW_PATH=$(brew --prefix)

cp gocommands.rb ${HOMEBREW_PATH}/Homebrew/Library/Taps/homebrew/homebrew-core/Formula/gocommands.rb
brew install --build-from-source --verbose --debug gocommands