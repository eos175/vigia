#define _POSIX_C_SOURCE 200809L

#include <signal.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

static void ignore_signal(int signo) {
	const char *name = "signal";

	if (signo == SIGINT) {
		name = "SIGINT";
	} else if (signo == SIGTERM) {
		name = "SIGTERM";
	}

	fprintf(stderr, "[%ld] ignoring %s\n", (long)getpid(), name);
}

static void install_handlers(void) {
	struct sigaction sa;

	sa.sa_handler = ignore_signal;
	sigemptyset(&sa.sa_mask);
	sa.sa_flags = SA_RESTART;

	if (sigaction(SIGINT, &sa, NULL) == -1) {
		perror("sigaction(SIGINT)");
		exit(1);
	}

	if (sigaction(SIGTERM, &sa, NULL) == -1) {
		perror("sigaction(SIGTERM)");
		exit(1);
	}
}

static void loop_forever(const char *role) {
	(void)role;

	for (;;) {
		sleep(1);
	}
}

int main(void) {
	setvbuf(stdout, NULL, _IONBF, 0);
	setvbuf(stderr, NULL, _IONBF, 0);

	pid_t pid = fork();
	if (pid < 0) {
		perror("fork");
		return 1;
	}

	if (pid == 0) {
		install_handlers();
		fprintf(stderr,
				"[child %ld] pgid=%ld ppid=%ld\n",
				(long)getpid(),
				(long)getpgrp(),
				(long)getppid());
		loop_forever("child");
	}

	install_handlers();
	fprintf(stderr,
			"[parent %ld] spawned child=%ld pgid=%ld\n",
			(long)getpid(),
			(long)pid,
			(long)getpgrp());
	loop_forever("parent");

	return 0;
}
