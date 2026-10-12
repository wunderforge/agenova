class Agenova < Formula
  desc "Claim-scoped agent execution CLI"
  homepage "https://github.com/wunderforge/agenova"
  url "https://github.com/wunderforge/agenova/archive/64395c9be7e2c2868616301ac6edb9d413c2e211.tar.gz"
  version "0.1.0-preview.20261012.1"
  sha256 "14359e53f8110c8e87704fc32b50269d190986dc49410fce00990bd750b9099b"
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
