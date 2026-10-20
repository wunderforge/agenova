class Agenova < Formula
  desc "Claim-scoped agent execution CLI"
  homepage "https://github.com/wunderforge/agenova"
  url "https://github.com/wunderforge/agenova/archive/3921df511f3d5d0d7425eaf0163997ee4d378450.tar.gz"
  version "0.1.0-preview.20261013.1"
  sha256 "1b4e197e56aa9ef5eabaeb688ea37d87a21d4e4edf9f68371b1243a7ad3cb4ec"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    # Native macOS Keychain support must remain enabled; Linux uses Secret Service.
    ENV["CGO_ENABLED"] = OS.mac? ? "1" : "0"
    ENV["GOTOOLCHAIN"] = "go1.27.2"
    system "go", "build", "-trimpath",
           "-ldflags=-X github.com/wunderforge/agenova/internal/cli.Version=#{version}",
           "-o", bin/"agenova", "./cmd/agenova"
  end

  def caveats
    <<~EOS
      This preview is built from PR #216, not an upstream stable release.
      Sign in: agenova login --server https://app.agenova.app
      New members need administrator approval. Credentials use macOS Keychain or Linux Secret Service.
      Linux/WSL: an unlocked Secret Service keyring and a D-Bus session are required.
      WSL2: install wslu for wslview, or use the native Windows CLI.
      Work commands need no AWS CLI, kubectl, Docker or cluster credentials.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/agenova --version")
    assert_match "work", shell_output("#{bin}/agenova --help")
  end
end
