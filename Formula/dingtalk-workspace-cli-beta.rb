class DingtalkWorkspaceCliBeta < Formula
  desc "Automate DingTalk workspace tasks from the terminal (beta channel)"
  homepage "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli"
  version "1.0.56-beta.1"
  license "Apache-2.0"
  keg_only "it is the beta channel and conflicts with dingtalk-workspace-cli"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.1/dws-darwin-arm64.tar.gz"
      sha256 "26b696a7807f87cf9c6a24634e08008bac0501f2664a85a8355904309068d55e"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.1/dws-darwin-amd64.tar.gz"
      sha256 "28daa9fa1fc8131e3567a39042d5f34efc61d5901bf7d88a67f075a4ff366be6"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.1/dws-linux-arm64.tar.gz"
      sha256 "5f9ead9b939d2db31354797cc7b7431f27aaca41f171d256d4d4e9269cc66d51"
    else
      url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.1/dws-linux-amd64.tar.gz"
      sha256 "06b950676cefdd8e054c76af32cd9758c4c7786908d654281d9c65e7a3d575cf"
    end
  end

  resource "skills" do
    url "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/releases/download/v1.0.56-beta.1/dws-skills.zip"
    sha256 "7448ccfc8e189b3d76b3e8f11f5dedc4f5a5b53380f9059e6af4c63a38e38f28"
  end

  def install
    root = Dir["dws-*"].find { |entry| File.directory?(entry) } || "."
    binary = File.join(root, "dws")
    raise "binary not found: #{binary}" unless File.exist?(binary)

    bin.install binary => "dws"

    %w[LICENSE NOTICE README.md CHANGELOG.md].each do |name|
      source = File.join(root, name)
      pkgshare.install source if File.exist?(source)
    end

    skill_dest = pkgshare/"skills/dws"
    skill_dest.mkpath
    resource("skills").stage do
      cp_r(Dir["*"], skill_dest)
    end
  end

  def caveats
    <<~EOS
      Agent Skills are bundled in #{pkgshare}/skills/dws.
      Run `dws skill setup` to install them into your Agent directories.
      This beta is keg-only. Add #{opt_bin} to PATH to use its `dws` binary.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/dws version")
  end
end
