/**
 * BPF Disassembler
 *
 * Copyright (c) 2012 Red Hat <pmoore@redhat.com>
 * Author: Paul Moore <pmoore@redhat.com>
 */

/*
 * This library is free software; you can redistribute it and/or modify it
 * under the terms of version 2.1 of the GNU Lesser General Public License as
 * published by the Free Software Foundation.
 *
 * This library is distributed in the hope that it will be useful, but WITHOUT
 * ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or
 * FITNESS FOR A PARTICULAR PURPOSE.  See the GNU Lesser General Public License
 * for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with this library; if not, see <http://www.gnu.org/licenses>.
 */

#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <limits.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <unistd.h>
#include <linux/audit.h>
#include <sys/types.h>
#include <sys/stat.h>

#include "bpf.h"
#include "util.h"

#define _OP_FMT			"%-3s"

/**
 * Decode the BPF operand
 * @param bpf the BPF instruction
 *
 * Decode the BPF operand and print it to stdout.
 *
 */
static void bpf_decode_op(const bpf_instr_raw *bpf)
{
	switch (bpf->code) {
	case BPF_LD+BPF_W+BPF_IMM:
	case BPF_LD+BPF_W+BPF_ABS:
	case BPF_LD+BPF_W+BPF_IND:
	case BPF_LD+BPF_W+BPF_MEM:
	case BPF_LD+BPF_W+BPF_LEN:
	case BPF_LD+BPF_W+BPF_MSH:
		printf(_OP_FMT, "ld");
		break;
	case BPF_LD+BPF_H+BPF_IMM:
	case BPF_LD+BPF_H+BPF_ABS:
	case BPF_LD+BPF_H+BPF_IND:
	case BPF_LD+BPF_H+BPF_MEM:
	case BPF_LD+BPF_H+BPF_LEN:
	case BPF_LD+BPF_H+BPF_MSH:
		printf(_OP_FMT, "ldh");
		break;
	case BPF_LD+BPF_B+BPF_IMM:
	case BPF_LD+BPF_B+BPF_ABS:
	case BPF_LD+BPF_B+BPF_IND:
	case BPF_LD+BPF_B+BPF_MEM:
	case BPF_LD+BPF_B+BPF_LEN:
	case BPF_LD+BPF_B+BPF_MSH:
		printf(_OP_FMT, "ldb");
		break;
	case BPF_LDX+BPF_W+BPF_IMM:
	case BPF_LDX+BPF_W+BPF_ABS:
	case BPF_LDX+BPF_W+BPF_IND:
	case BPF_LDX+BPF_W+BPF_MEM:
	case BPF_LDX+BPF_W+BPF_LEN:
	case BPF_LDX+BPF_W+BPF_MSH:
	case BPF_LDX+BPF_H+BPF_IMM:
	case BPF_LDX+BPF_H+BPF_ABS:
	case BPF_LDX+BPF_H+BPF_IND:
	case BPF_LDX+BPF_H+BPF_MEM:
	case BPF_LDX+BPF_H+BPF_LEN:
	case BPF_LDX+BPF_H+BPF_MSH:
	case BPF_LDX+BPF_B+BPF_IMM:
	case BPF_LDX+BPF_B+BPF_ABS:
	case BPF_LDX+BPF_B+BPF_IND:
	case BPF_LDX+BPF_B+BPF_MEM:
	case BPF_LDX+BPF_B+BPF_LEN:
	case BPF_LDX+BPF_B+BPF_MSH:
		printf(_OP_FMT, "ldx");
		break;
	case BPF_ST:
		printf(_OP_FMT, "st");
		break;
	case BPF_STX:
		printf(_OP_FMT, "stx");
		break;
	case BPF_ALU+BPF_ADD+BPF_K:
	case BPF_ALU+BPF_ADD+BPF_X:
		printf(_OP_FMT, "add");
		break;
	case BPF_ALU+BPF_SUB+BPF_K:
	case BPF_ALU+BPF_SUB+BPF_X:
		printf(_OP_FMT, "sub");
		break;
	case BPF_ALU+BPF_MUL+BPF_K:
	case BPF_ALU+BPF_MUL+BPF_X:
		printf(_OP_FMT, "mul");
		break;
	case BPF_ALU+BPF_DIV+BPF_K:
	case BPF_ALU+BPF_DIV+BPF_X:
		printf(_OP_FMT, "div");
		break;
	case BPF_ALU+BPF_OR+BPF_K:
	case BPF_ALU+BPF_OR+BPF_X:
		printf(_OP_FMT, "or");
		break;
	case BPF_ALU+BPF_AND+BPF_K:
	case BPF_ALU+BPF_AND+BPF_X:
		printf(_OP_FMT, "and");
		break;
	case BPF_ALU+BPF_LSH+BPF_K:
	case BPF_ALU+BPF_LSH+BPF_X:
		printf(_OP_FMT, "lsh");
		break;
	case BPF_ALU+BPF_RSH+BPF_K:
	case BPF_ALU+BPF_RSH+BPF_X:
		printf(_OP_FMT, "rsh");
		break;
	case BPF_ALU+BPF_NEG+BPF_K:
	case BPF_ALU+BPF_NEG+BPF_X:
		printf(_OP_FMT, "neg");
		break;
	case BPF_JMP+BPF_JA+BPF_K:
	case BPF_JMP+BPF_JA+BPF_X:
		printf(_OP_FMT, "jmp");
		break;
	case BPF_JMP+BPF_JEQ+BPF_K:
	case BPF_JMP+BPF_JEQ+BPF_X:
		printf(_OP_FMT, "jeq");
		break;
	case BPF_JMP+BPF_JGT+BPF_K:
	case BPF_JMP+BPF_JGT+BPF_X:
		printf(_OP_FMT, "jgt");
		break;
	case BPF_JMP+BPF_JGE+BPF_K:
	case BPF_JMP+BPF_JGE+BPF_X:
		printf(_OP_FMT, "jge");
		break;
	case BPF_JMP+BPF_JSET+BPF_K:
	case BPF_JMP+BPF_JSET+BPF_X:
		printf(_OP_FMT, "jset");
		break;
	case BPF_RET+BPF_K:
	case BPF_RET+BPF_X:
	case BPF_RET+BPF_A:
		printf(_OP_FMT, "ret");
		break;
	case BPF_MISC+BPF_TAX:
		printf(_OP_FMT, "tax");
		break;
	case BPF_MISC+BPF_TXA:
		printf(_OP_FMT, "txa");
		break;
	default:
		printf(_OP_FMT, "???");
	}
}

/**
 * Decode the BPF arguments (JT, JF, and K)
 * @param bpf the BPF instruction
 * @param line the current line number
 *
 * Decode the BPF arguments (JT, JF, and K) and print the relevant information
 * to stdout based on the operand.
 *
 */
static void bpf_decode_args(const bpf_instr_raw *bpf, unsigned int line)
{
	switch (BPF_CLASS(bpf->code)) {
	case BPF_LD:
	case BPF_LDX:
		switch (BPF_MODE(bpf->code)) {
		case BPF_ABS:
			printf("$data[%u]", bpf->k);
			break;
		case BPF_MEM:
			printf("$temp[%u]", bpf->k);
			break;
		}
		break;
	case BPF_ST:
	case BPF_STX:
		printf("$temp[%u]", bpf->k);
		break;
	case BPF_ALU:
		if (BPF_SRC(bpf->code) == BPF_K) {
			switch (BPF_OP(bpf->code)) {
			case BPF_OR:
			case BPF_AND:
				printf("0x%.8x", bpf->k);
				break;
			default:
				printf("%u", bpf->k);
			}
		} else
			printf("%u", bpf->k);
		break;
	case BPF_JMP:
		if (BPF_OP(bpf->code) == BPF_JA) {
			printf("%.4u", (line + 1) + bpf->k);
		} else {
			printf("%-4u true:%.4u false:%.4u",
			       bpf->k,
			       (line + 1) + bpf->jt,
			       (line + 1) + bpf->jf);
		}
		break;
	case BPF_RET:
		if (BPF_RVAL(bpf->code) == BPF_A) {
			/* XXX - accumulator? */
			printf("$acc");
		} else if (BPF_SRC(bpf->code) == BPF_K) {
			uint32_t act = bpf->k & SECCOMP_RET_ACTION;
			uint32_t data = bpf->k & SECCOMP_RET_DATA;

			switch (act) {
			case SECCOMP_RET_KILL:
				printf("KILL");
				break;
			case SECCOMP_RET_TRAP:
				printf("TRAP");
				break;
			case SECCOMP_RET_ERRNO:
				printf("ERRNO(%u)", data);
				break;
			case SECCOMP_RET_TRACE:
				printf("TRACE(%u)", data);
				break;
			case SECCOMP_RET_ALLOW:
				printf("ALLOW");
				break;
			default:
				printf("0x%.8x", bpf->k);
			}
		} else if (BPF_SRC(bpf->code) == BPF_X) {
			/* XXX - any idea? */
			printf("???");
		}
		break;
	case BPF_MISC:
		break;
	default:
		printf("???");
	}
}

/**
 * Perform a simple decoding of the BPF program
 * @param file the BPF program
 *
 * Read the BPF program and display the instructions.  Returns zero on success,
 * negative values on failure.
 *
 */
static int bpf_decode(FILE *file)
{
	unsigned int line = 0;
	size_t len;
	bpf_instr_raw bpf;

	/* header */
	printf(" line  OP   JT   JF   K\n");
	printf("=================================\n");

	while ((len = fread(&bpf, sizeof(bpf), 1, file))) {
		/* convert the bpf statement */
		bpf.code = ttoh16(arch, bpf.code);
		bpf.k = ttoh32(arch, bpf.k);

		/* display a hex dump */
		printf(" %.4u: 0x%.2x 0x%.2x 0x%.2x 0x%.8x",
		       line, bpf.code, bpf.jt, bpf.jf, bpf.k);

		/* display the assembler statements */
		printf("   ");
		bpf_decode_op(&bpf);
		printf(" ");
		bpf_decode_args(&bpf, line);
		printf("\n");

		line++;
	}

	if (ferror(file))
		return errno;
	return 0;
}

/**
 * main
 */
int main(int argc, char *argv[])
{
	int rc;
	int opt;
	FILE *file;

	/* parse the command line */
	while ((opt = getopt(argc, argv, "a:h")) > 0) {
		switch (opt) {
		case 'a':
			if (strcmp(optarg, "x86") == 0)
				arch = AUDIT_ARCH_I386;
			else if (strcmp(optarg, "x86_64") == 0)
				arch = AUDIT_ARCH_X86_64;
			else if (strcmp(optarg, "x32") == 0)
				arch = AUDIT_ARCH_X86_64;
			else if (strcmp(optarg, "arm") == 0)
				arch = AUDIT_ARCH_ARM;
			else if (strcmp(optarg, "aarch64") == 0)
				arch = AUDIT_ARCH_AARCH64;
			else if (strcmp(optarg, "mips") == 0)
				arch = AUDIT_ARCH_MIPS;
			else if (strcmp(optarg, "mipsel") == 0)
				arch = AUDIT_ARCH_MIPSEL;
			else if (strcmp(optarg, "mips64") == 0)
				arch = AUDIT_ARCH_MIPS64;
			else if (strcmp(optarg, "mipsel64") == 0)
				arch = AUDIT_ARCH_MIPSEL64;
			else if (strcmp(optarg, "mips64n32") == 0)
				arch = AUDIT_ARCH_MIPS64N32;
			else if (strcmp(optarg, "mipsel64n32") == 0)
				arch = AUDIT_ARCH_MIPSEL64N32;
			else
				exit_usage(argv[0]);
			break;
		default:
			/* usage information */
			exit_usage(argv[0]);
		}
	}

	if ((optind > 1) && (optind < argc)) {
		int opt_file = optind - 1 ;
		file = fopen(argv[opt_file], "r");
		if (file == NULL) {
			fprintf(stderr, "error: unable to open \"%s\" (%s)\n",
				argv[opt_file], strerror(errno));
			return errno;
		}
	} else
		file = stdin;

	rc = bpf_decode(file);
	fclose(file);

	return rc;
}
