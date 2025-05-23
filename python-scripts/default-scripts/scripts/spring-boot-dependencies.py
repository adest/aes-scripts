#!../.venv/bin/python

import json
import subprocess
import shutil
from pathlib import Path
from xml.etree import ElementTree as ET
import xml.etree.ElementTree as ET
import argparse
import json
import requests

TEMP_BOOT_DIR = Path("spring-boot-temp-project")
TEMP_CLOUD_DIR = Path("spring-cloud-temp-project")


POM_BOOT_TEMPLATE = """<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>

  <groupId>com.example</groupId>
  <artifactId>spring-temp</artifactId>
  <version>1.0.0</version>
  <packaging>jar</packaging>

  <parent>
    <groupId>org.springframework.boot</groupId>
    <artifactId>spring-boot-starter-parent</artifactId>
    <version>{version}</version>
    <relativePath/> <!-- lookup parent from repository -->
  </parent>

  <dependencies>
    <dependency>
      <groupId>org.springframework.boot</groupId>
      <artifactId>spring-boot-starter-web</artifactId>
    </dependency>
  </dependencies>

</project>
"""

POM_CLOUD_TEMPLATE = """<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>

  <groupId>com.example</groupId>
  <artifactId>spring-temp</artifactId>
  <version>1.0.0</version>
  <packaging>jar</packaging>

  <parent>
    <groupId>org.springframework.cloud</groupId>
    <artifactId>spring-cloud-dependencies</artifactId>
    <version>{version}</version>
    <relativePath/> <!-- lookup parent from repository -->
  </parent>

  <dependencies>
    <dependency>
      <groupId>org.springframework.cloud</groupId>
      <artifactId>spring-cloud-starter</artifactId>
    </dependency>
  </dependencies>

</project>
"""


def create_project_structure(tmp_dir, template, version):
    if tmp_dir.exists():
        shutil.rmtree(tmp_dir)
    tmp_dir.mkdir(parents=True)
    with open(tmp_dir / "pom.xml", "w") as f:
        f.write(template.format(version=version))


def run_maven_dependency_tree(tmp_dir):
    result = subprocess.run(
        ["mvn", "dependency:tree", "-DoutputType=text"],
        cwd=tmp_dir,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True
    )
    if result.returncode != 0:
        print("Erreur Maven:\n", result.stderr)
        return None
    return result.stdout


def extract_dependencies(mvn_output):
    deps = set()
    for line in mvn_output.splitlines():
        if '---' in line:
            continue
        if ':' in line:
            parts = line.strip().split(":")
            if len(parts) >= 5:
                group_id = parts[0]
                artifact_id = parts[1]
                version = parts[3]
                deps.add(f"{group_id}:{artifact_id}:{version}")
    return sorted(deps)


def print_dependency_version(deps: set, name: str, filter: str):
    for dep in deps:
        if filter in dep:
            print(f"  - Version de {name} : {dep.split(':')[2]}")
            return
    print(f"  ❌ Aucune version trouvée pour {filter} dans les dépendances.")


def get_latest_version(group_id, artifact_id):
    base_url = "https://search.maven.org/solrsearch/select"
    query = f'g:{group_id} AND a:{artifact_id}'
    params = {
        "q": query,
        "rows": 1,
        "wt": "json"
    }
    try:
        response = requests.get(base_url, params=params, timeout=10)
        response.raise_for_status()
        data = response.json()
        docs = data.get("response", {}).get("docs", [])
        if not docs:
            raise Exception(f"Aucune version trouvée pour {group_id}:{artifact_id}")
        return docs[0].get("latestVersion")
    except Exception as e:
        print(f"❌ Erreur lors de la récupération de la version : {e}")
        return None



def get_dependency_version(group_id, artifact_id, version, dep_group_id, dep_artifact_id):
    # Maven Central POM URL
    group_path = group_id.replace('.', '/')
    pom_url = f"https://repo1.maven.org/maven2/{group_path}/{artifact_id}/{version}/{artifact_id}-{version}.pom"
    try:
        response = requests.get(pom_url, timeout=10)
        response.raise_for_status()
        pom_xml = response.content
        root = ET.fromstring(pom_xml)
        ns = {'m': 'http://maven.apache.org/POM/4.0.0'}
        for dep in root.findall('.//m:dependency', ns):
            g = dep.find('m:groupId', ns)
            a = dep.find('m:artifactId', ns)
            v = dep.find('m:version', ns)
            if g is not None and a is not None and g.text == dep_group_id and a.text == dep_artifact_id:
                return v.text if v is not None else None
        return None
    except Exception as e:
        print(f"Erreur lors de la récupération du POM : {e}")
        return None


def parse_arguments() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        description="Lister les dépendances transitives d'une version de Spring Boot."
    )
    parser.add_argument(
        "--spring-boot", "-sb",
        dest="boot",
        type=str,
        help="La version de Spring Boot à utiliser (ex: 3.4.5)",
    )
    parser.add_argument(
        "--spring-cloud", "-sc",
        dest="cloud",
        type=str,
        help="La version de Spring Cloud à utiliser (ex: 2024.0.1)",
    )
    parser.add_argument(
        "--keep", "-k",
        dest="keep",
        help="Garde les répertoires temporaires après l'exécution",
    )
    return parser


def main():
    parser = parse_arguments()
    args = parser.parse_args()
    if not args.boot:
        import sys
        parser.print_help()
        sys.exit(0)

    print("📦 Dépendances depuis maven central :")
    compress_group = "org.apache.commons"
    compress_artifact = "commons-compress"
    compress_version = get_latest_version(compress_group, compress_artifact)
    print(f"  - Derniere version de commons-compress disponible: {compress_version}")
    io_version = get_dependency_version(compress_group, compress_artifact, compress_version, "commons-io", "commons-io")
    print(f"  - Version de commons-io: {io_version}")

    print("")
    spring_version = args.boot
    print(f"🔧 Génération du projet Spring Boot {spring_version}")
    create_project_structure(TEMP_BOOT_DIR, POM_BOOT_TEMPLATE, spring_version)

    print("🚀 Exécution de Maven pour obtenir les dépendances...")
    boot_dependencies = run_maven_dependency_tree(TEMP_BOOT_DIR)
    if boot_dependencies is None:
        return

    print("📦 Dépendances résolues :")
    dependencies = extract_dependencies(boot_dependencies)
    print_dependency_version(dependencies, "Spring framework", "org.springframework:spring-context")
    print_dependency_version(dependencies, "Jackson", "com.fasterxml.jackson.core:jackson-core")
    print_dependency_version(dependencies, "Log4J", "org.apache.logging.log4j:log4j-api")

    if args.cloud:
        spring_cloud_version = args.cloud
        print("")
        print(f"🔧 Génération du projet Spring Cloud {spring_cloud_version}")
        create_project_structure(TEMP_CLOUD_DIR, POM_CLOUD_TEMPLATE, spring_cloud_version)

        print("🚀 Exécution de Maven pour obtenir les dépendances...")
        cloud_dependencies = run_maven_dependency_tree(TEMP_CLOUD_DIR)
        if cloud_dependencies is None:
            return
        
        print("📦 Dépendances résolues :")
        dependencies = extract_dependencies(cloud_dependencies)
        print_dependency_version(dependencies, "Bouncy Castle", "org.bouncycastle")

    if not args.keep:
        print("🗑️ Suppression des répertoires temporaires...")
        if TEMP_BOOT_DIR.exists():
            shutil.rmtree(TEMP_BOOT_DIR)
        if TEMP_CLOUD_DIR.exists():
            shutil.rmtree(TEMP_CLOUD_DIR)


if __name__ == "__main__":
    main()