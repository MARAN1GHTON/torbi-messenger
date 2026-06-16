import os
import sys
import time
import subprocess
import threading
import queue
import sqlite3

# Configuration
NODE1_DB = "test_node1.db"
NODE2_DB = "test_node2.db"
NODE1_PASS = "secure_password_node_1"
NODE2_PASS = "secure_password_node_2"
NODE1_PORT = 11001
NODE2_PORT = 11002

def enqueue_output(out, q):
    try:
        for line in iter(out.readline, b''):
            q.put(line.decode('utf-8', errors='replace'))
    except Exception as e:
        pass
    finally:
        out.close()

def wait_for_log(q, substring, timeout=10, name="Node"):
    start = time.time()
    accumulated = []
    while time.time() - start < timeout:
        try:
            line = q.get(timeout=0.1)
            clean_line = line.strip()
            if clean_line:
                print(f"[{name}] {clean_line}")
                accumulated.append(clean_line)
            if substring in line:
                return True, accumulated
        except queue.Empty:
            continue
    return False, accumulated

def main():
    print("==========================================================")
    print("          TORBI P2P MESSENGER INTEGRATION TEST            ")
    print("==========================================================")

    # 1. Clean up old databases
    for db in [NODE1_DB, NODE2_DB]:
        if os.path.exists(db):
            print(f"Cleaning up old database file: {db}")
            try:
                os.remove(db)
            except Exception as e:
                print(f"Warning: Could not remove {db}: {e}")

    # 2. Build the binary
    print("\n[Step 1] Compiling Torbi binary using MSYS2 Go...")
    env = os.environ.copy()
    env["MSYSTEM"] = "UCRT64"
    env["CHERE_INVOKING"] = "1"
    
    try:
        # Run go build via MSYS2 bash to ensure standard environment alignment
        res = subprocess.run(
            ["C:\\msys64\\usr\\bin\\bash.exe", "-lc", "go build -o torbi.exe"],
            env=env,
            capture_output=True,
            text=True
        )
        if res.returncode != 0:
            print("Compilation failed!")
            print("stdout:", res.stdout)
            print("stderr:", res.stderr)
            sys.exit(1)
        print("Compilation successful! Created torbi.exe")
    except Exception as e:
        print(f"Error executing compiler command: {e}")
        sys.exit(1)

    # 3. Launch Node 1 and Node 2 (redirecting stderr to stdout to prevent deadlock and see all logs)
    print("\n[Step 2] Launching Node 1 and Node 2...")
    
    # Node 1 Command
    cmd1 = ["./torbi.exe", "-db", NODE1_DB, "-pass", NODE1_PASS, "-port", str(NODE1_PORT)]
    node1 = subprocess.Popen(cmd1, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    
    # Node 2 Command
    cmd2 = ["./torbi.exe", "-db", NODE2_DB, "-pass", NODE2_PASS, "-port", str(NODE2_PORT)]
    node2 = subprocess.Popen(cmd2, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)

    # Start output reader threads
    q1 = queue.Queue()
    t1 = threading.Thread(target=enqueue_output, args=(node1.stdout, q1), daemon=True)
    t1.start()

    q2 = queue.Queue()
    t2 = threading.Thread(target=enqueue_output, args=(node2.stdout, q2), daemon=True)
    t2.start()

    # Wait for Node 1 initialization
    print("\nWaiting for Node 1 to initialize...")
    success1, logs1 = wait_for_log(q1, "Type /help to see all available commands.", timeout=10, name="Node 1")
    if not success1:
        print("Failed to initialize Node 1 within timeout!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # Wait for Node 2 initialization
    print("\nWaiting for Node 2 to initialize...")
    success2, logs2 = wait_for_log(q2, "Type /help to see all available commands.", timeout=10, name="Node 2")
    if not success2:
        print("Failed to initialize Node 2 within timeout!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # Extract PeerIDs and loopback multiaddresses
    node1_id = None
    node1_addr = None
    for log in logs1:
        if "PeerID:" in log:
            node1_id = log.split("PeerID:")[1].strip()
        if "/ip4/127.0.0.1/tcp/" in log:
            node1_addr = log.strip()

    node2_id = None
    node2_addr = None
    for log in logs2:
        if "PeerID:" in log:
            node2_id = log.split("PeerID:")[1].strip()
        if "/ip4/127.0.0.1/tcp/" in log:
            node2_addr = log.strip()

    print(f"\n[Parsed Node Identities]")
    print(f"Node 1 ID: {node1_id}")
    print(f"Node 1 Local Multiaddress: {node1_addr}")
    print(f"Node 2 ID: {node2_id}")
    print(f"Node 2 Local Multiaddress: {node2_addr}")

    if not node1_id or not node2_addr:
        print("Failed to parse Node IDs/Addresses!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # 4. Connect Node 1 to Node 2
    print(f"\n[Step 3] Connecting Node 1 to Node 2...")
    connect_cmd = f"/connect {node2_addr}\n".encode()
    node1.stdin.write(connect_cmd)
    node1.stdin.flush()

    # Wait for connection log on Node 1
    success_conn, _ = wait_for_log(q1, "Connection established successfully.", timeout=10, name="Node 1")
    if not success_conn:
        print("Nodes failed to connect!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # Wait an extra second for handshakes and sync to trigger
    print("Waiting 2 seconds for P2P Handshakes to settle...")
    time.sleep(2)

    # 5. Open chat room on both nodes (with retries to handle async handshake completion)
    print(f"\n[Step 4] Opening chat room on Node 1 with Node 2...")
    node1_chat_success = False
    for attempt in range(5):
        node1.stdin.write(f"/chat {node2_id}\n".encode())
        node1.stdin.flush()
        success, _ = wait_for_log(q1, "Switched to chat session", timeout=2, name="Node 1")
        if success:
            node1_chat_success = True
            break
        print("Node 1 handshake might still be finalizing, retrying in 1s...")
        time.sleep(1)

    if not node1_chat_success:
        print("Failed to open chat room on Node 1!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    print(f"\nOpening chat room on Node 2 with Node 1...")
    node2_chat_success = False
    for attempt in range(5):
        node2.stdin.write(f"/chat {node1_id}\n".encode())
        node2.stdin.flush()
        success, _ = wait_for_log(q2, "Switched to chat session", timeout=2, name="Node 2")
        if success:
            node2_chat_success = True
            break
        print("Node 2 handshake might still be finalizing, retrying in 1s...")
        time.sleep(1)

    if not node2_chat_success:
        print("Failed to open chat room on Node 2!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # 6. Exchange real-time encrypted messages
    print(f"\n[Step 5] Sending message from Node 1 to Node 2...")
    test_msg_from_1 = "Hello from Node 1! Secure channel check."
    node1.stdin.write(f"{test_msg_from_1}\n".encode())
    node1.stdin.flush()

    # Wait for Node 2 to output the message
    success_msg2, _ = wait_for_log(q2, test_msg_from_1, timeout=5, name="Node 2")
    if not success_msg2:
        print("Message from Node 1 was not received by Node 2!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    print(f"\nSending reply from Node 2 to Node 1...")
    test_msg_from_2 = "Hello back! E2EE is working fine."
    node2.stdin.write(f"{test_msg_from_2}\n".encode())
    node2.stdin.flush()

    # Wait for Node 1 to output the message
    success_msg1, _ = wait_for_log(q1, test_msg_from_2, timeout=5, name="Node 1")
    if not success_msg1:
        print("Message from Node 2 was not received by Node 1!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    print("\nE2EE Messaging verified successfully.")

    # 7. Verify local database encryption
    print("\n[Step 6] Verifying SQLite database encryption...")
    try:
        # Try to open the database file using standard sqlite3 driver (lacking SQLCipher decryption)
        conn = sqlite3.connect(NODE1_DB)
        cursor = conn.cursor()
        cursor.execute("SELECT name FROM sqlite_master;")
        cursor.fetchall()
        print("WARNING: Standard sqlite3 was able to query the database! It might not be encrypted.")
        db_encrypted = False
    except sqlite3.DatabaseError as e:
        if "file is not a database" in str(e):
            print("SUCCESS: Standard SQLite parser failed to read the database file (file is encrypted).")
            db_encrypted = True
        else:
            print(f"Unknown sqlite3 error: {e}")
            db_encrypted = False
    finally:
        try:
            conn.close()
        except:
            pass

    if not db_encrypted:
        print("Database encryption verification FAILED!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # 8. Test Offline Message Synchronization / Replication
    print("\n[Step 7] Testing offline history replication (black box sync)...")
    
    # Close chats and exit Node 2
    print("Closing chat sessions...")
    node1.stdin.write(b"/close\n")
    node1.stdin.flush()
    node2.stdin.write(b"/close\n")
    node2.stdin.flush()
    
    time.sleep(1)
    
    print("Shutting down Node 2...")
    node2.stdin.write(b"/exit\n")
    node2.stdin.flush()
    
    # Wait for Node 2 process to exit
    try:
        node2.wait(timeout=5)
        print("Node 2 shut down successfully.")
    except subprocess.TimeoutExpired:
        print("Node 2 failed to exit, forcing termination.")
        node2.terminate()

    # Open chat on Node 1 again and write offline messages
    print("\nGenerating offline messages on Node 1...")
    node1.stdin.write(f"/chat {node2_id}\n".encode())
    node1.stdin.flush()
    wait_for_log(q1, "Switched to chat session", timeout=5, name="Node 1")

    offline_msg_1 = "Offline Message A - Queued while Node 2 was down."
    offline_msg_2 = "Offline Message B - Verifying Lamport clock sequence."

    node1.stdin.write(f"{offline_msg_1}\n".encode())
    node1.stdin.flush()
    time.sleep(0.5)
    node1.stdin.write(f"{offline_msg_2}\n".encode())
    node1.stdin.flush()
    
    time.sleep(1)
    print("Offline messages saved to Node 1's local database.")
    
    node1.stdin.write(b"/close\n")
    node1.stdin.flush()

    # Relaunch Node 2
    print("\nRelaunching Node 2...")
    node2 = subprocess.Popen(cmd2, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    q2 = queue.Queue()
    t2 = threading.Thread(target=enqueue_output, args=(node2.stdout, q2), daemon=True)
    t2.start()

    success2, _ = wait_for_log(q2, "Type /help to see all available commands.", timeout=10, name="Node 2")
    if not success2:
        print("Failed to re-initialize Node 2!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # Reconnect Node 1 to Node 2 to trigger sync
    print("\nReconnecting Node 1 to Node 2...")
    node1.stdin.write(f"/connect {node2_addr}\n".encode())
    node1.stdin.flush()

    # Wait for Sync completed message
    print("\nWaiting for history synchronization protocol to complete...")
    sync_done, _ = wait_for_log(q1, "Synchronized chat history with peer:", timeout=15, name="Node 1")
    if not sync_done:
        # Also check Node 2's logs
        sync_done, _ = wait_for_log(q2, "Synchronized chat history with peer:", timeout=5, name="Node 2")
        
    if not sync_done:
        print("Synchronization protocol failed to complete within timeout!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    print("Synchronization protocol completed successfully!")

    # Check history on Node 2
    print("\n[Step 8] Verifying replicated history on Node 2...")
    node2_chat_success = False
    for attempt in range(5):
        node2.stdin.write(f"/chat {node1_id}\n".encode())
        node2.stdin.flush()
        success, _ = wait_for_log(q2, "Switched to chat session", timeout=2, name="Node 2")
        if success:
            node2_chat_success = True
            break
        print("Node 2 handshake might still be finalizing, retrying in 1s...")
        time.sleep(1)

    if not node2_chat_success:
        print("Failed to enter chat on Node 2!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    node2.stdin.write(b"/history\n")
    node2.stdin.flush()

    # Wait for the history dump and look for the offline messages
    print("\nVerifying presence of offline messages in Node 2's history:")
    found_msg_1, _ = wait_for_log(q2, offline_msg_1, timeout=5, name="Node 2 History")
    found_msg_2, _ = wait_for_log(q2, offline_msg_2, timeout=5, name="Node 2 History")

    if found_msg_1 and found_msg_2:
        print("\nSUCCESS: All offline messages successfully synchronized, decrypted, and verified on Node 2!")
    else:
        print("\nFAILURE: Replicated messages not found in Node 2 history!")
        node1.terminate()
        node2.terminate()
        sys.exit(1)

    # Clean shut down
    print("\nShutting down nodes...")
    node1.stdin.write(b"/exit\n")
    node1.stdin.flush()
    node2.stdin.write(b"/exit\n")
    node2.stdin.flush()

    try:
        node1.wait(timeout=5)
        node2.wait(timeout=5)
        print("Both nodes exited cleanly.")
    except:
        node1.terminate()
        node2.terminate()

    print("\n==========================================================")
    print("        ALL INTEGRATION TESTS PASSED SUCCESSFULLY!         ")
    print("==========================================================")

if __name__ == "__main__":
    main()
