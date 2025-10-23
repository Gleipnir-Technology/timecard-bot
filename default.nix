{ pkgs ? import <nixpkgs> { } }:
pkgs.buildGoModule rec {
	meta = {
		description = "FieldSeeker sync";
		homepage = "https://github.com/Gleipnir-Technology/fieldseeker-sync";
	};
	pname = "fieldseeker-sync";
	src = ./.;
	subPackages = [
		"cmd/convert-completed-tasks"
		"cmd/download-schema"
		"cmd/dump"
		"cmd/full-export"
		"cmd/login"
		"cmd/migrate"
		"cmd/registration"
		"cmd/webserver"
	];
	version = "0.0.31";
	# Needs to be updated after every modification of go.mod/go.sum
	vendorHash = "sha256-eKe1StY4PFW0wBOakajLH2C/tyqkqsuOS0glWnTQkc0=";
}
