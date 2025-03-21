// Code generated by "stringer -type=FeatureID,Vendor"; DO NOT EDIT.

package cpuid

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[ADX-1]
	_ = x[AESNI-2]
	_ = x[AMD3DNOW-3]
	_ = x[AMD3DNOWEXT-4]
	_ = x[AMXBF16-5]
	_ = x[AMXFP16-6]
	_ = x[AMXINT8-7]
	_ = x[AMXFP8-8]
	_ = x[AMXTILE-9]
	_ = x[APX_F-10]
	_ = x[AVX-11]
	_ = x[AVX10-12]
	_ = x[AVX10_128-13]
	_ = x[AVX10_256-14]
	_ = x[AVX10_512-15]
	_ = x[AVX2-16]
	_ = x[AVX512BF16-17]
	_ = x[AVX512BITALG-18]
	_ = x[AVX512BW-19]
	_ = x[AVX512CD-20]
	_ = x[AVX512DQ-21]
	_ = x[AVX512ER-22]
	_ = x[AVX512F-23]
	_ = x[AVX512FP16-24]
	_ = x[AVX512IFMA-25]
	_ = x[AVX512PF-26]
	_ = x[AVX512VBMI-27]
	_ = x[AVX512VBMI2-28]
	_ = x[AVX512VL-29]
	_ = x[AVX512VNNI-30]
	_ = x[AVX512VP2INTERSECT-31]
	_ = x[AVX512VPOPCNTDQ-32]
	_ = x[AVXIFMA-33]
	_ = x[AVXNECONVERT-34]
	_ = x[AVXSLOW-35]
	_ = x[AVXVNNI-36]
	_ = x[AVXVNNIINT8-37]
	_ = x[AVXVNNIINT16-38]
	_ = x[BHI_CTRL-39]
	_ = x[BMI1-40]
	_ = x[BMI2-41]
	_ = x[CETIBT-42]
	_ = x[CETSS-43]
	_ = x[CLDEMOTE-44]
	_ = x[CLMUL-45]
	_ = x[CLZERO-46]
	_ = x[CMOV-47]
	_ = x[CMPCCXADD-48]
	_ = x[CMPSB_SCADBS_SHORT-49]
	_ = x[CMPXCHG8-50]
	_ = x[CPBOOST-51]
	_ = x[CPPC-52]
	_ = x[CX16-53]
	_ = x[EFER_LMSLE_UNS-54]
	_ = x[ENQCMD-55]
	_ = x[ERMS-56]
	_ = x[F16C-57]
	_ = x[FLUSH_L1D-58]
	_ = x[FMA3-59]
	_ = x[FMA4-60]
	_ = x[FP128-61]
	_ = x[FP256-62]
	_ = x[FSRM-63]
	_ = x[FXSR-64]
	_ = x[FXSROPT-65]
	_ = x[GFNI-66]
	_ = x[HLE-67]
	_ = x[HRESET-68]
	_ = x[HTT-69]
	_ = x[HWA-70]
	_ = x[HYBRID_CPU-71]
	_ = x[HYPERVISOR-72]
	_ = x[IA32_ARCH_CAP-73]
	_ = x[IA32_CORE_CAP-74]
	_ = x[IBPB-75]
	_ = x[IBPB_BRTYPE-76]
	_ = x[IBRS-77]
	_ = x[IBRS_PREFERRED-78]
	_ = x[IBRS_PROVIDES_SMP-79]
	_ = x[IBS-80]
	_ = x[IBSBRNTRGT-81]
	_ = x[IBSFETCHSAM-82]
	_ = x[IBSFFV-83]
	_ = x[IBSOPCNT-84]
	_ = x[IBSOPCNTEXT-85]
	_ = x[IBSOPSAM-86]
	_ = x[IBSRDWROPCNT-87]
	_ = x[IBSRIPINVALIDCHK-88]
	_ = x[IBS_FETCH_CTLX-89]
	_ = x[IBS_OPDATA4-90]
	_ = x[IBS_OPFUSE-91]
	_ = x[IBS_PREVENTHOST-92]
	_ = x[IBS_ZEN4-93]
	_ = x[IDPRED_CTRL-94]
	_ = x[INT_WBINVD-95]
	_ = x[INVLPGB-96]
	_ = x[KEYLOCKER-97]
	_ = x[KEYLOCKERW-98]
	_ = x[LAHF-99]
	_ = x[LAM-100]
	_ = x[LBRVIRT-101]
	_ = x[LZCNT-102]
	_ = x[MCAOVERFLOW-103]
	_ = x[MCDT_NO-104]
	_ = x[MCOMMIT-105]
	_ = x[MD_CLEAR-106]
	_ = x[MMX-107]
	_ = x[MMXEXT-108]
	_ = x[MOVBE-109]
	_ = x[MOVDIR64B-110]
	_ = x[MOVDIRI-111]
	_ = x[MOVSB_ZL-112]
	_ = x[MOVU-113]
	_ = x[MPX-114]
	_ = x[MSRIRC-115]
	_ = x[MSRLIST-116]
	_ = x[MSR_PAGEFLUSH-117]
	_ = x[NRIPS-118]
	_ = x[NX-119]
	_ = x[OSXSAVE-120]
	_ = x[PCONFIG-121]
	_ = x[POPCNT-122]
	_ = x[PPIN-123]
	_ = x[PREFETCHI-124]
	_ = x[PSFD-125]
	_ = x[RDPRU-126]
	_ = x[RDRAND-127]
	_ = x[RDSEED-128]
	_ = x[RDTSCP-129]
	_ = x[RRSBA_CTRL-130]
	_ = x[RTM-131]
	_ = x[RTM_ALWAYS_ABORT-132]
	_ = x[SBPB-133]
	_ = x[SERIALIZE-134]
	_ = x[SEV-135]
	_ = x[SEV_64BIT-136]
	_ = x[SEV_ALTERNATIVE-137]
	_ = x[SEV_DEBUGSWAP-138]
	_ = x[SEV_ES-139]
	_ = x[SEV_RESTRICTED-140]
	_ = x[SEV_SNP-141]
	_ = x[SGX-142]
	_ = x[SGXLC-143]
	_ = x[SHA-144]
	_ = x[SME-145]
	_ = x[SME_COHERENT-146]
	_ = x[SPEC_CTRL_SSBD-147]
	_ = x[SRBDS_CTRL-148]
	_ = x[SRSO_MSR_FIX-149]
	_ = x[SRSO_NO-150]
	_ = x[SRSO_USER_KERNEL_NO-151]
	_ = x[SSE-152]
	_ = x[SSE2-153]
	_ = x[SSE3-154]
	_ = x[SSE4-155]
	_ = x[SSE42-156]
	_ = x[SSE4A-157]
	_ = x[SSSE3-158]
	_ = x[STIBP-159]
	_ = x[STIBP_ALWAYSON-160]
	_ = x[STOSB_SHORT-161]
	_ = x[SUCCOR-162]
	_ = x[SVM-163]
	_ = x[SVMDA-164]
	_ = x[SVMFBASID-165]
	_ = x[SVML-166]
	_ = x[SVMNP-167]
	_ = x[SVMPF-168]
	_ = x[SVMPFT-169]
	_ = x[SYSCALL-170]
	_ = x[SYSEE-171]
	_ = x[TBM-172]
	_ = x[TDX_GUEST-173]
	_ = x[TLB_FLUSH_NESTED-174]
	_ = x[TME-175]
	_ = x[TOPEXT-176]
	_ = x[TSCRATEMSR-177]
	_ = x[TSXLDTRK-178]
	_ = x[VAES-179]
	_ = x[VMCBCLEAN-180]
	_ = x[VMPL-181]
	_ = x[VMSA_REGPROT-182]
	_ = x[VMX-183]
	_ = x[VPCLMULQDQ-184]
	_ = x[VTE-185]
	_ = x[WAITPKG-186]
	_ = x[WBNOINVD-187]
	_ = x[WRMSRNS-188]
	_ = x[X87-189]
	_ = x[XGETBV1-190]
	_ = x[XOP-191]
	_ = x[XSAVE-192]
	_ = x[XSAVEC-193]
	_ = x[XSAVEOPT-194]
	_ = x[XSAVES-195]
	_ = x[AESARM-196]
	_ = x[ARMCPUID-197]
	_ = x[ASIMD-198]
	_ = x[ASIMDDP-199]
	_ = x[ASIMDHP-200]
	_ = x[ASIMDRDM-201]
	_ = x[ATOMICS-202]
	_ = x[CRC32-203]
	_ = x[DCPOP-204]
	_ = x[EVTSTRM-205]
	_ = x[FCMA-206]
	_ = x[FP-207]
	_ = x[FPHP-208]
	_ = x[GPA-209]
	_ = x[JSCVT-210]
	_ = x[LRCPC-211]
	_ = x[PMULL-212]
	_ = x[SHA1-213]
	_ = x[SHA2-214]
	_ = x[SHA3-215]
	_ = x[SHA512-216]
	_ = x[SM3-217]
	_ = x[SM4-218]
	_ = x[SVE-219]
	_ = x[lastID-220]
	_ = x[firstID-0]
}

const _FeatureID_name = "firstIDADXAESNIAMD3DNOWAMD3DNOWEXTAMXBF16AMXFP16AMXINT8AMXFP8AMXTILEAPX_FAVXAVX10AVX10_128AVX10_256AVX10_512AVX2AVX512BF16AVX512BITALGAVX512BWAVX512CDAVX512DQAVX512ERAVX512FAVX512FP16AVX512IFMAAVX512PFAVX512VBMIAVX512VBMI2AVX512VLAVX512VNNIAVX512VP2INTERSECTAVX512VPOPCNTDQAVXIFMAAVXNECONVERTAVXSLOWAVXVNNIAVXVNNIINT8AVXVNNIINT16BHI_CTRLBMI1BMI2CETIBTCETSSCLDEMOTECLMULCLZEROCMOVCMPCCXADDCMPSB_SCADBS_SHORTCMPXCHG8CPBOOSTCPPCCX16EFER_LMSLE_UNSENQCMDERMSF16CFLUSH_L1DFMA3FMA4FP128FP256FSRMFXSRFXSROPTGFNIHLEHRESETHTTHWAHYBRID_CPUHYPERVISORIA32_ARCH_CAPIA32_CORE_CAPIBPBIBPB_BRTYPEIBRSIBRS_PREFERREDIBRS_PROVIDES_SMPIBSIBSBRNTRGTIBSFETCHSAMIBSFFVIBSOPCNTIBSOPCNTEXTIBSOPSAMIBSRDWROPCNTIBSRIPINVALIDCHKIBS_FETCH_CTLXIBS_OPDATA4IBS_OPFUSEIBS_PREVENTHOSTIBS_ZEN4IDPRED_CTRLINT_WBINVDINVLPGBKEYLOCKERKEYLOCKERWLAHFLAMLBRVIRTLZCNTMCAOVERFLOWMCDT_NOMCOMMITMD_CLEARMMXMMXEXTMOVBEMOVDIR64BMOVDIRIMOVSB_ZLMOVUMPXMSRIRCMSRLISTMSR_PAGEFLUSHNRIPSNXOSXSAVEPCONFIGPOPCNTPPINPREFETCHIPSFDRDPRURDRANDRDSEEDRDTSCPRRSBA_CTRLRTMRTM_ALWAYS_ABORTSBPBSERIALIZESEVSEV_64BITSEV_ALTERNATIVESEV_DEBUGSWAPSEV_ESSEV_RESTRICTEDSEV_SNPSGXSGXLCSHASMESME_COHERENTSPEC_CTRL_SSBDSRBDS_CTRLSRSO_MSR_FIXSRSO_NOSRSO_USER_KERNEL_NOSSESSE2SSE3SSE4SSE42SSE4ASSSE3STIBPSTIBP_ALWAYSONSTOSB_SHORTSUCCORSVMSVMDASVMFBASIDSVMLSVMNPSVMPFSVMPFTSYSCALLSYSEETBMTDX_GUESTTLB_FLUSH_NESTEDTMETOPEXTTSCRATEMSRTSXLDTRKVAESVMCBCLEANVMPLVMSA_REGPROTVMXVPCLMULQDQVTEWAITPKGWBNOINVDWRMSRNSX87XGETBV1XOPXSAVEXSAVECXSAVEOPTXSAVESAESARMARMCPUIDASIMDASIMDDPASIMDHPASIMDRDMATOMICSCRC32DCPOPEVTSTRMFCMAFPFPHPGPAJSCVTLRCPCPMULLSHA1SHA2SHA3SHA512SM3SM4SVElastID"

var _FeatureID_index = [...]uint16{0, 7, 10, 15, 23, 34, 41, 48, 55, 61, 68, 73, 76, 81, 90, 99, 108, 112, 122, 134, 142, 150, 158, 166, 173, 183, 193, 201, 211, 222, 230, 240, 258, 273, 280, 292, 299, 306, 317, 329, 337, 341, 345, 351, 356, 364, 369, 375, 379, 388, 406, 414, 421, 425, 429, 443, 449, 453, 457, 466, 470, 474, 479, 484, 488, 492, 499, 503, 506, 512, 515, 518, 528, 538, 551, 564, 568, 579, 583, 597, 614, 617, 627, 638, 644, 652, 663, 671, 683, 699, 713, 724, 734, 749, 757, 768, 778, 785, 794, 804, 808, 811, 818, 823, 834, 841, 848, 856, 859, 865, 870, 879, 886, 894, 898, 901, 907, 914, 927, 932, 934, 941, 948, 954, 958, 967, 971, 976, 982, 988, 994, 1004, 1007, 1023, 1027, 1036, 1039, 1048, 1063, 1076, 1082, 1096, 1103, 1106, 1111, 1114, 1117, 1129, 1143, 1153, 1165, 1172, 1191, 1194, 1198, 1202, 1206, 1211, 1216, 1221, 1226, 1240, 1251, 1257, 1260, 1265, 1274, 1278, 1283, 1288, 1294, 1301, 1306, 1309, 1318, 1334, 1337, 1343, 1353, 1361, 1365, 1374, 1378, 1390, 1393, 1403, 1406, 1413, 1421, 1428, 1431, 1438, 1441, 1446, 1452, 1460, 1466, 1472, 1480, 1485, 1492, 1499, 1507, 1514, 1519, 1524, 1531, 1535, 1537, 1541, 1544, 1549, 1554, 1559, 1563, 1567, 1571, 1577, 1580, 1583, 1586, 1592}

func (i FeatureID) String() string {
	if i < 0 || i >= FeatureID(len(_FeatureID_index)-1) {
		return "FeatureID(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _FeatureID_name[_FeatureID_index[i]:_FeatureID_index[i+1]]
}
func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[VendorUnknown-0]
	_ = x[Intel-1]
	_ = x[AMD-2]
	_ = x[VIA-3]
	_ = x[Transmeta-4]
	_ = x[NSC-5]
	_ = x[KVM-6]
	_ = x[MSVM-7]
	_ = x[VMware-8]
	_ = x[XenHVM-9]
	_ = x[Bhyve-10]
	_ = x[Hygon-11]
	_ = x[SiS-12]
	_ = x[RDC-13]
	_ = x[Ampere-14]
	_ = x[ARM-15]
	_ = x[Broadcom-16]
	_ = x[Cavium-17]
	_ = x[DEC-18]
	_ = x[Fujitsu-19]
	_ = x[Infineon-20]
	_ = x[Motorola-21]
	_ = x[NVIDIA-22]
	_ = x[AMCC-23]
	_ = x[Qualcomm-24]
	_ = x[Marvell-25]
	_ = x[QEMU-26]
	_ = x[QNX-27]
	_ = x[ACRN-28]
	_ = x[SRE-29]
	_ = x[Apple-30]
	_ = x[lastVendor-31]
}

const _Vendor_name = "VendorUnknownIntelAMDVIATransmetaNSCKVMMSVMVMwareXenHVMBhyveHygonSiSRDCAmpereARMBroadcomCaviumDECFujitsuInfineonMotorolaNVIDIAAMCCQualcommMarvellQEMUQNXACRNSREApplelastVendor"

var _Vendor_index = [...]uint8{0, 13, 18, 21, 24, 33, 36, 39, 43, 49, 55, 60, 65, 68, 71, 77, 80, 88, 94, 97, 104, 112, 120, 126, 130, 138, 145, 149, 152, 156, 159, 164, 174}

func (i Vendor) String() string {
	if i < 0 || i >= Vendor(len(_Vendor_index)-1) {
		return "Vendor(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Vendor_name[_Vendor_index[i]:_Vendor_index[i+1]]
}
