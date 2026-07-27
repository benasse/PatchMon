import { describe, expect, it } from "vitest";
import {
	buildOSSecuritySummary,
	getOSSecuritySupport,
	normalizeOS,
	OS_SECURITY_STATUS,
} from "../utils/osSecuritySupport";

const now = new Date("2026-07-27T12:00:00Z");

describe("osSecuritySupport", () => {
	it("normalizes Ubuntu-style host reports", () => {
		const normalized = normalizeOS("Ubuntu", "22.04 LTS");

		expect(normalized).toMatchObject({
			family: "linux",
			distro: "ubuntu",
			displayName: "Ubuntu",
			version: "22.04",
			majorVersion: "22",
		});
	});

	it("classifies Ubuntu 22.04 as active standard security support", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Ubuntu", os_version: "22.04 LTS" },
			now,
		);

		expect(result.securitySupport.status).toBe(
			OS_SECURITY_STATUS.SECURITY_ACTIVE,
		);
		expect(result.securitySupport.supportEndsAt).toBe("2027-05-31");
	});

	it("adds the Ubuntu 26.04 release name", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Ubuntu", os_version: "26.04 LTS" },
			now,
		);

		expect(result.normalized).toMatchObject({
			distro: "ubuntu",
			version: "26.04",
			codename: "resolute",
		});
		expect(result.securitySupport.supportEndsAt).toBe("2031-05-31");
	});

	it("classifies former extended-available cases as security EOL", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Ubuntu", os_version: "20.04 LTS" },
			now,
		);

		expect(result.securitySupport.status).toBe(OS_SECURITY_STATUS.SECURITY_EOL);
		expect(result.securitySupport.reason).toContain("paid extended support");
	});

	it("classifies Debian LTS as security maintenance without ELTS", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Debian GNU/Linux", os_version: "11 (bullseye)" },
			now,
		);

		expect(result.normalized).toMatchObject({
			distro: "debian",
			version: "11",
			codename: "bullseye",
		});
		expect(result.securitySupport.status).toBe(
			OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		);
	});

	it("classifies Debian 10 as EOL because ELTS is ignored in v1", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Debian GNU/Linux", os_version: "10 (buster)" },
			now,
		);

		expect(result.securitySupport.status).toBe(OS_SECURITY_STATUS.SECURITY_EOL);
		expect(result.securitySupport.reason).toContain("ELTS ignored");
	});

	it("classifies older Debian releases with their release names", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Debian GNU/Linux", os_version: "8" },
			now,
		);

		expect(result.normalized).toMatchObject({
			distro: "debian",
			version: "8",
			codename: "jessie",
		});
		expect(result.securitySupport.status).toBe(OS_SECURITY_STATUS.SECURITY_EOL);
	});

	it("classifies RHEL-like distributions by product and major version", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Rocky Linux", os_version: "9.4" },
			now,
		);

		expect(result.normalized).toMatchObject({
			distro: "rocky",
			displayName: "Rocky Linux",
			version: "9",
			codename: "blue onyx",
		});
		expect(result.securitySupport.status).toBe(
			OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		);
	});

	it("classifies Arch-like distributions as rolling", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Arch Linux", os_version: "rolling" },
			now,
		);

		expect(result.securitySupport.status).toBe(OS_SECURITY_STATUS.ROLLING);
	});

	it("classifies unsupported Alpine branches as EOL", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Alpine Linux", os_version: "3.20.7" },
			now,
		);

		expect(result.securitySupport.status).toBe(OS_SECURITY_STATUS.SECURITY_EOL);
	});

	it("classifies non-Linux hosts separately", () => {
		const result = getOSSecuritySupport(
			{ os_type: "Windows", os_version: "10.0.20348" },
			now,
		);

		expect(result.securitySupport.status).toBe(OS_SECURITY_STATUS.NON_LINUX);
	});

	it("builds summary counts", () => {
		const summary = buildOSSecuritySummary(
			[
				{ os_type: "Ubuntu", os_version: "24.04 LTS" },
				{ os_type: "Ubuntu", os_version: "20.04 LTS" },
				{ os_type: "Arch Linux", os_version: "rolling" },
			],
			now,
		);

		expect(summary.totalHosts).toBe(3);
		expect(summary.linuxHosts).toBe(3);
		expect(summary.counts.security_active).toBe(1);
		expect(summary.counts.security_eol).toBe(1);
		expect(summary.counts.rolling).toBe(1);
	});
});
