import subprocess
import json
from typing import List, Dict

def run_performence_test(exe_file: str) -> float:
    result = subprocess.run(
        [exe_file],
        capture_output=True,
        text=True,  # Decodes stdout/stderr as text using default encoding
        check=True  # Raises CalledProcessError if the command returns a non-zero exit code
    )

    if "error" in result.stdout:
        raise RuntimeError(f"Executable got error: {result.stdout}")

    return float(result.stdout.strip())


def run__executable_performence_tests(exe_file: str) -> List[float]:
    results: List[float] = []
    for i in range(5):
        print(f"Running {exe_file} performence test #{i+1}")
        try:
            result = run_performence_test(exe_file)
            print(f"Got result: {result} MB in 10 seconds")
            results.append(result)
        except Exception as e:
            print(f"Got Error, Retrying, error: {e}")
            i-=1

    results.sort()
    return results


def performence_tests() -> Dict[str, List[float]]:
    exe_files = ["main", "clock", "option", "context", "set-cap-and-load-atomic"]
    results: Dict[str, List[float]] = {}
    for exe_file in exe_files:
        result = run__executable_performence_tests(exe_file)
        results[exe_file] = result
    
    print(results)
    return results


def save_results(results: Dict[str, List[float]]) -> None:
    filename = "results.json"
    with open(filename, 'w') as f:
        json.dump(results, f, indent=4) # indent for pretty-printing
    print(f"Dictionary successfully saved to {filename}")


def main():
    save_results(performence_tests())


if __name__ == "__main__":
    main()