export const OS_SECURITY_STATUS = {
	SECURITY_ACTIVE: "security_active",
	SECURITY_MAINTENANCE: "security_maintenance",
	SECURITY_EOL: "security_eol",
	ROLLING: "rolling",
	UNKNOWN: "unknown",
	NON_LINUX: "non_linux",
};

export const OS_SECURITY_STATUS_META = {
	[OS_SECURITY_STATUS.SECURITY_ACTIVE]: {
		label: "Security active",
		shortLabel: "Active",
		description: "Standard security fixes are available.",
	},
	[OS_SECURITY_STATUS.SECURITY_MAINTENANCE]: {
		label: "Security maintenance",
		shortLabel: "Maintenance",
		description: "Security fixes are available in maintenance/LTS mode.",
	},
	[OS_SECURITY_STATUS.SECURITY_EOL]: {
		label: "Security EOL",
		shortLabel: "EOL",
		description:
			"No standard security support is known. Paid vendor or third-party subscriptions may exist but are not verified by PatchMon.",
	},
	[OS_SECURITY_STATUS.ROLLING]: {
		label: "Rolling",
		shortLabel: "Rolling",
		description: "Rolling release without a fixed end-of-life date.",
	},
	[OS_SECURITY_STATUS.UNKNOWN]: {
		label: "Unknown",
		shortLabel: "Unknown",
		description: "PatchMon could not confidently classify this OS/version.",
	},
	[OS_SECURITY_STATUS.NON_LINUX]: {
		label: "Non Linux",
		shortLabel: "Non Linux",
		description: "Outside the Linux support scope for this report.",
	},
};

const SOURCES = {
	ubuntuReleaseCycle: {
		label: "Ubuntu release cycle",
		url: "https://ubuntu.com/about/release-cycle",
	},
	ubuntuReleases: {
		label: "Ubuntu releases",
		url: "https://ubuntu.com/project/docs/release-team/list-of-releases/",
	},
	debianReleases: {
		label: "Debian releases",
		url: "https://www.debian.org/releases/",
	},
	rhelLifecycle: {
		label: "Red Hat Enterprise Linux lifecycle",
		url: "https://access.redhat.com/support/policy/updates/errata",
	},
	rockyReleaseNotes: {
		label: "Rocky Linux version guide",
		url: "https://wiki.rockylinux.org/rocky/version/",
	},
	almaReleaseNotes: {
		label: "AlmaLinux release notes",
		url: "https://wiki.almalinux.org/release-notes/",
	},
	oracleLifecycle: {
		label: "Oracle Linux lifecycle",
		url: "https://www.oracle.com/a/ocom/docs/elsp-lifetime-069338.pdf",
	},
	amazonLinuxFaq: {
		label: "Amazon Linux lifecycle",
		url: "https://aws.amazon.com/linux/amazon-linux-2023/faqs/",
	},
	suseLifecycle: {
		label: "SUSE lifecycle",
		url: "https://www.suse.com/lifecycle/",
	},
	fedoraEol: {
		label: "Fedora EOL support",
		url: "https://fedoraproject.org/wiki/Fedora-EOL-Support",
	},
	alpineReleases: {
		label: "Alpine releases",
		url: "https://www.alpinelinux.org/releases/",
	},
	archAbout: {
		label: "Arch Linux about",
		url: "https://archlinux.org/about/",
	},
};

const RULES = [
	rule(
		"ubuntu",
		"Ubuntu",
		"26.04",
		"resolute",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2031-05-31",
		"Standard security maintenance",
		[SOURCES.ubuntuReleaseCycle, SOURCES.ubuntuReleases],
	),
	rule(
		"ubuntu",
		"Ubuntu",
		"24.04",
		"noble",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2029-05-31",
		"Standard security maintenance",
		[SOURCES.ubuntuReleaseCycle, SOURCES.ubuntuReleases],
	),
	rule(
		"ubuntu",
		"Ubuntu",
		"22.04",
		"jammy",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2027-05-31",
		"Standard security maintenance",
		[SOURCES.ubuntuReleaseCycle, SOURCES.ubuntuReleases],
	),
	rule(
		"ubuntu",
		"Ubuntu",
		"20.04",
		"focal",
		OS_SECURITY_STATUS.SECURITY_EOL,
		"2025-05-31",
		"Standard support ended; paid extended support may exist",
		[SOURCES.ubuntuReleaseCycle, SOURCES.ubuntuReleases],
	),
	rule(
		"debian",
		"Debian",
		"13",
		"trixie",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2028-08-31",
		"Stable security support",
		[SOURCES.debianReleases],
	),
	rule(
		"debian",
		"Debian",
		"12",
		"bookworm",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2028-06-30",
		"Debian LTS",
		[SOURCES.debianReleases],
	),
	rule(
		"debian",
		"Debian",
		"11",
		"bullseye",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2026-08-31",
		"Debian LTS",
		[SOURCES.debianReleases],
	),
	rule(
		"debian",
		"Debian",
		"10",
		"buster",
		OS_SECURITY_STATUS.SECURITY_EOL,
		"2024-06-30",
		"ELTS ignored in v1",
		[SOURCES.debianReleases],
	),
	rule(
		"debian",
		"Debian",
		"9",
		"stretch",
		OS_SECURITY_STATUS.SECURITY_EOL,
		"2022-06-30",
		"ELTS ignored in v1",
		[SOURCES.debianReleases],
	),
	rule(
		"debian",
		"Debian",
		"8",
		"jessie",
		OS_SECURITY_STATUS.SECURITY_EOL,
		"2020-06-30",
		"ELTS ignored in v1",
		[SOURCES.debianReleases],
	),
	rule(
		"rhel",
		"Red Hat Enterprise Linux",
		"10",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2035-05-31",
		"Full support until 2030-05-31; security maintenance afterward",
		[SOURCES.rhelLifecycle],
	),
	rule(
		"rhel",
		"Red Hat Enterprise Linux",
		"9",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2032-05-31",
		"Security/maintenance phase",
		[SOURCES.rhelLifecycle],
	),
	rule(
		"rhel",
		"Red Hat Enterprise Linux",
		"8",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2029-05-31",
		"Security/maintenance phase",
		[SOURCES.rhelLifecycle],
	),
	rule(
		"rocky",
		"Rocky Linux",
		"10",
		"red quartz",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2035-05-31",
		"Full support until 2030-05-31; security maintenance afterward",
		[SOURCES.rockyReleaseNotes],
	),
	rule(
		"rocky",
		"Rocky Linux",
		"9",
		"blue onyx",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2032-05-31",
		"Security/maintenance phase",
		[SOURCES.rockyReleaseNotes],
	),
	rule(
		"rocky",
		"Rocky Linux",
		"8",
		"green obsidian",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2029-05-31",
		"Security/maintenance phase",
		[SOURCES.rockyReleaseNotes],
	),
	rule(
		"alma",
		"AlmaLinux",
		"10",
		"purple lion",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2035-05-31",
		"Full support until 2030-05-31; security maintenance afterward",
		[SOURCES.almaReleaseNotes],
	),
	rule(
		"alma",
		"AlmaLinux",
		"9",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2032-05-31",
		"Security/maintenance phase",
		[SOURCES.almaReleaseNotes],
	),
	rule(
		"alma",
		"AlmaLinux",
		"8",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2029-05-31",
		"Security/maintenance phase",
		[SOURCES.almaReleaseNotes],
	),
	rule(
		"oracle",
		"Oracle Linux",
		"10",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2035-06-30",
		"Basic/Premier support",
		[SOURCES.oracleLifecycle],
	),
	rule(
		"oracle",
		"Oracle Linux",
		"9",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2032-06-30",
		"Basic/Premier support",
		[SOURCES.oracleLifecycle],
	),
	rule(
		"oracle",
		"Oracle Linux",
		"8",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2029-07-31",
		"Basic/Premier support",
		[SOURCES.oracleLifecycle],
	),
	rule(
		"amazon",
		"Amazon Linux",
		"2023",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2029-06-30",
		"Maintenance/security fixes",
		[SOURCES.amazonLinuxFaq],
	),
	rule(
		"amazon",
		"Amazon Linux",
		"2",
		"",
		OS_SECURITY_STATUS.SECURITY_EOL,
		"2026-06-30",
		"End of standard support",
		[SOURCES.amazonLinuxFaq],
	),
	rule(
		"fedora",
		"Fedora",
		"44",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2027-06-30",
		"Maintained release",
		[SOURCES.fedoraEol],
	),
	rule(
		"fedora",
		"Fedora",
		"43",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2026-12-31",
		"Maintained release",
		[SOURCES.fedoraEol],
	),
	rule(
		"alpine",
		"Alpine",
		"3.24",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2028-06-30",
		"Supported branch",
		[SOURCES.alpineReleases],
	),
	rule(
		"alpine",
		"Alpine",
		"3.23",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2027-11-30",
		"Supported branch",
		[SOURCES.alpineReleases],
	),
	rule(
		"alpine",
		"Alpine",
		"3.22",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2027-05-31",
		"Supported branch",
		[SOURCES.alpineReleases],
	),
	rule(
		"alpine",
		"Alpine",
		"3.21",
		"",
		OS_SECURITY_STATUS.SECURITY_MAINTENANCE,
		"2026-11-30",
		"Limited branch support",
		[SOURCES.alpineReleases],
	),
	rule(
		"sles",
		"SLES",
		"15.7",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"2031-07-31",
		"General support",
		[SOURCES.suseLifecycle],
	),
	rule(
		"sles",
		"SLES",
		"16",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"",
		"General support",
		[SOURCES.suseLifecycle],
	),
	rule(
		"opensuse",
		"openSUSE Leap",
		"16.0",
		"",
		OS_SECURITY_STATUS.SECURITY_ACTIVE,
		"",
		"Current Leap",
		[SOURCES.suseLifecycle],
	),
	rule(
		"arch",
		"Arch",
		"rolling",
		"",
		OS_SECURITY_STATUS.ROLLING,
		"",
		"Rolling release",
		[SOURCES.archAbout],
	),
];

function rule(
	distro,
	displayName,
	version,
	codename,
	status,
	supportEndsAt,
	note,
	sources,
) {
	return {
		distro,
		displayName,
		version,
		codename,
		status,
		supportEndsAt,
		note,
		sources,
	};
}

const toLower = (value) =>
	String(value || "")
		.trim()
		.toLowerCase();

const firstVersion = (value) => {
	const match = String(value || "").match(/\d+(?:\.\d+)?/);
	return match ? match[0] : "";
};

const majorVersion = (value) => firstVersion(value).split(".")[0] || "";

const normalizeRhelVersion = (value) => majorVersion(value);

export function normalizeOS(osType, osVersion) {
	const os = toLower(osType);
	const versionText = String(osVersion || "").trim();
	const version = firstVersion(versionText);
	const major = majorVersion(versionText);
	const codename = (versionText.match(/\(([^)]+)\)/)?.[1] || "").toLowerCase();

	if (!os || os === "unknown") {
		return unknownLinux(versionText);
	}

	if (
		os.includes("windows") ||
		os.includes("freebsd") ||
		os.includes("pfsense") ||
		os.includes("mac") ||
		os.includes("darwin")
	) {
		return {
			family: "non_linux",
			distro: "non_linux",
			displayName: osType || "Non Linux",
			version,
			majorVersion: major,
			codename,
		};
	}

	if (
		os.includes("arch") ||
		os.includes("manjaro") ||
		os.includes("endeavour")
	) {
		return linux("arch", "Arch", "rolling", "", "");
	}
	if (
		os.includes("ubuntu") ||
		os.includes("pop") ||
		os.includes("mint") ||
		os.includes("zorin")
	) {
		return linux("ubuntu", "Ubuntu", version, major, codename);
	}
	if (os.includes("debian") || os.includes("kali") || os.includes("proxmox")) {
		return linux("debian", "Debian", major || version, major, codename);
	}
	if (os.includes("amazon linux")) {
		const amazonVersion =
			os.includes("2023") || version === "2023" ? "2023" : major;
		return linux(
			"amazon",
			"Amazon Linux",
			amazonVersion,
			amazonVersion,
			codename,
		);
	}
	if (os.includes("rocky")) {
		const rhelVersion = normalizeRhelVersion(versionText || osType);
		return linux("rocky", "Rocky Linux", rhelVersion, rhelVersion, codename);
	}
	if (os.includes("alma")) {
		const rhelVersion = normalizeRhelVersion(versionText || osType);
		return linux("alma", "AlmaLinux", rhelVersion, rhelVersion, codename);
	}
	if (os.includes("oracle linux")) {
		const rhelVersion = normalizeRhelVersion(versionText || osType);
		return linux("oracle", "Oracle Linux", rhelVersion, rhelVersion, codename);
	}
	if (os.includes("rhel") || os.includes("red hat")) {
		const rhelVersion = normalizeRhelVersion(versionText || osType);
		return linux(
			"rhel",
			"Red Hat Enterprise Linux",
			rhelVersion,
			rhelVersion,
			codename,
		);
	}
	if (os.includes("centos")) {
		const rhelVersion = normalizeRhelVersion(versionText || osType);
		return linux(
			"unknown",
			"CentOS Stream",
			rhelVersion,
			rhelVersion,
			codename,
		);
	}
	if (os.includes("fedora")) {
		return linux("fedora", "Fedora", major || version, major, codename);
	}
	if (os.includes("alpine")) {
		return linux("alpine", "Alpine", version, major, codename);
	}
	if (os.includes("opensuse")) {
		return linux("opensuse", "openSUSE Leap", version, major, codename);
	}
	if (os.includes("sles") || os.includes("suse linux enterprise")) {
		const slesVersion =
			version === "15" && versionText.toLowerCase().includes("sp7")
				? "15.7"
				: version;
		return linux("sles", "SLES", slesVersion, major, codename);
	}
	if (os.includes("suse")) {
		return linux("opensuse", "openSUSE Leap", version, major, codename);
	}
	if (os.includes("linux")) {
		return unknownLinux(versionText);
	}

	return unknownLinux(versionText);
}

function linux(distro, displayName, version, major, codename) {
	return {
		family: "linux",
		distro,
		displayName,
		version: version || "",
		majorVersion: major || majorVersion(version),
		codename: codename || "",
	};
}

function unknownLinux(versionText) {
	const version = firstVersion(versionText);
	return linux("unknown", "Unknown Linux", version, majorVersion(version), "");
}

export function getOSSecuritySupport(host, now = new Date()) {
	const normalized = normalizeOS(host?.os_type, host?.os_version);

	if (normalized.family === "non_linux") {
		return supportResult(
			normalized,
			OS_SECURITY_STATUS.NON_LINUX,
			"",
			"Out of Linux scope",
			[],
		);
	}

	if (normalized.distro === "unknown") {
		return supportResult(
			normalized,
			OS_SECURITY_STATUS.UNKNOWN,
			"",
			"Version not recognized",
			[],
		);
	}

	const matched = findRule(normalized);
	if (!matched) {
		if (normalized.distro === "fedora" && Number(normalized.version) <= 42) {
			return supportResult(
				normalized,
				OS_SECURITY_STATUS.SECURITY_EOL,
				"",
				"No standard security updates",
				[SOURCES.fedoraEol],
			);
		}
		if (
			normalized.distro === "alpine" &&
			compareVersions(normalized.version, "3.20") <= 0
		) {
			return supportResult(
				normalized,
				OS_SECURITY_STATUS.SECURITY_EOL,
				"",
				"Expired/on-request support ignored in v1",
				[SOURCES.alpineReleases],
			);
		}
		return supportResult(
			normalized,
			OS_SECURITY_STATUS.UNKNOWN,
			"",
			"Version not recognized",
			[],
		);
	}

	const computedStatus = applyDateFloor(
		matched.status,
		matched.supportEndsAt,
		now,
	);
	return supportResult(
		{
			...normalized,
			codename: normalized.codename || matched.codename,
		},
		computedStatus,
		matched.supportEndsAt,
		matched.note,
		matched.sources,
	);
}

function findRule(normalized) {
	if (normalized.distro === "sles" && normalized.version.startsWith("16")) {
		return RULES.find((r) => r.distro === "sles" && r.version === "16");
	}
	return RULES.find(
		(r) => r.distro === normalized.distro && r.version === normalized.version,
	);
}

function applyDateFloor(status, supportEndsAt, now) {
	if (
		!supportEndsAt ||
		status === OS_SECURITY_STATUS.ROLLING ||
		status === OS_SECURITY_STATUS.SECURITY_EOL
	) {
		return status;
	}
	const end = new Date(`${supportEndsAt}T23:59:59Z`);
	return end < now ? OS_SECURITY_STATUS.SECURITY_EOL : status;
}

function supportResult(normalized, status, supportEndsAt, reason, sources) {
	return {
		normalized,
		securitySupport: {
			status,
			label: OS_SECURITY_STATUS_META[status]?.label || "Unknown",
			supportEndsAt: supportEndsAt || null,
			reason,
			sources,
		},
	};
}

function compareVersions(a, b) {
	const left = String(a || "")
		.split(".")
		.map((v) => Number(v) || 0);
	const right = String(b || "")
		.split(".")
		.map((v) => Number(v) || 0);
	const len = Math.max(left.length, right.length);
	for (let i = 0; i < len; i += 1) {
		if ((left[i] || 0) > (right[i] || 0)) return 1;
		if ((left[i] || 0) < (right[i] || 0)) return -1;
	}
	return 0;
}

export function buildOSSecuritySummary(hosts, now = new Date()) {
	const rows = (Array.isArray(hosts) ? hosts : []).map((host) => ({
		...host,
		...getOSSecuritySupport(host, now),
	}));
	const counts = Object.fromEntries(
		Object.values(OS_SECURITY_STATUS).map((status) => [status, 0]),
	);
	for (const row of rows) {
		counts[row.securitySupport.status] =
			(counts[row.securitySupport.status] || 0) + 1;
	}
	return {
		rows,
		counts,
		totalHosts: rows.length,
		linuxHosts: rows.filter((row) => row.normalized.family === "linux").length,
	};
}

export function getOSSecuritySupportReference() {
	return [
		...RULES,
		rule(
			"fedora",
			"Fedora",
			"<=42",
			"",
			OS_SECURITY_STATUS.SECURITY_EOL,
			"",
			"No standard security updates",
			[SOURCES.fedoraEol],
		),
		rule(
			"alpine",
			"Alpine",
			"<=3.20",
			"",
			OS_SECURITY_STATUS.SECURITY_EOL,
			"",
			"Expired/on-request ignored",
			[SOURCES.alpineReleases],
		),
		rule(
			"unknown",
			"Unknown Linux",
			"unknown",
			"",
			OS_SECURITY_STATUS.UNKNOWN,
			"",
			"Version not recognized",
			[],
		),
		rule(
			"non_linux",
			"Windows/BSD/macOS",
			"any",
			"",
			OS_SECURITY_STATUS.NON_LINUX,
			"",
			"Out of Linux scope",
			[],
		),
	];
}
