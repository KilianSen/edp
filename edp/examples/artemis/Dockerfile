# All-in-one, self-updating Artemis testing image.
# One container runs native MySQL + clones/builds/runs Artemis via ./gradlew
# bootRun, and a watcher rebuilds on new commits to ARTEMIS_BRANCH. See
# entrypoint.sh. Build context is tiny — the source is cloned at *runtime*.
FROM eclipse-temurin:25-jdk

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=en_US.UTF-8 LANGUAGE=en_US:en LC_ALL=en_US.UTF-8

# Baked defaults — override per deploy (edp run_env: ARTEMIS_BRANCH=…).
ENV ARTEMIS_REPO_URL=https://github.com/KilianSen/Artemis.git \
    ARTEMIS_BRANCH=feature/math/scaffold \
    ARTEMIS_PROFILES=artemis,scheduling,core,dev

# Toolchain the entrypoint needs: native MySQL + git + Artemis build deps.
# (The Gradle build downloads its own Node/npm, so no system node here.)
RUN set -eux; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
        git curl ca-certificates gnupg graphviz \
        mysql-server mysql-client locales; \
    sed -i 's/^# *en_US.UTF-8/en_US.UTF-8/' /etc/locale.gen && locale-gen; \
    rm -rf /var/lib/apt/lists/*

COPY zz-artemis.cnf /etc/mysql/mysql.conf.d/zz-artemis.cnf
COPY --chmod=755 entrypoint.sh /entrypoint.sh

EXPOSE 5005 8080
ENTRYPOINT ["/entrypoint.sh"]
