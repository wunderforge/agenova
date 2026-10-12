class Agenova < Formula
  desc "Claim-scoped agent execution CLI"
  homepage "https://github.com/wunderforge/agenova"
  url "https://github.com/wunderforge/agenova/archive/02d4adb67f28c5092db037468d8096e909b8acbb.tar.gz"
  version "0.1.0-preview.20261012"
  sha256 "6e2af0cc2adecace57754e27b29a3b946efa6c1cdfe58e58091bab21a474221f"
  license "Apache-2.0"

  depends_on "go" => :build
  depends_on :macos

  def install
    # Native macOS Keychain support must remain enabled.
    ENV["CGO_ENABLED"] = "1"
    ENV["GOTOOLCHAIN"] = "go1.27.2"
    system "go", "build", "-trimpath",
           "-ldflags=-X github.com/wunderforge/agenova/internal/cli.Version=#{version}",
           "-o", bin/"agenova", "./cmd/agenova"
  end

  def caveats
    <<~EOS
      This preview is built from PR #216, not an upstream stable release.
      Sign in: agenova login --server https://app.agenova.app
      New members need administrator approval. Credentials use macOS Keychain.
      Work commands need no AWS CLI, kubectl, Docker or cluster credentials.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/agenova --version")
    assert_match "work", shell_output("#{bin}/agenova --help")
  end
end
