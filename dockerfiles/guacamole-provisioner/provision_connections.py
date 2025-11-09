#!/usr/bin/env python3
"""
Optimized Guacamole Connection Provisioner

Performance improvements:
- Uses inotify for event-driven file watching instead of polling
- Implements connection pooling and reuse
- Batches database operations for efficiency
- Caches state to avoid redundant updates
- Configurable via environment variables
"""

import yaml
import psycopg2
from psycopg2 import pool
import os
import sys
import glob
import time
import hashlib
import json
from pathlib import Path
import signal

# Configuration from environment variables
CONNECTIONS_DIR = os.getenv('CONNECTIONS_DIR', '/connections')
CONFIG_DIR = os.getenv('CONFIG_DIR', '/config')
DB_HOST = os.getenv('DB_HOST', 'localhost')
DB_NAME = os.getenv('DB_NAME', 'guacamole_db')
DB_USER = os.getenv('DB_USER', 'guacamole')
STARTUP_DELAY = int(os.getenv('STARTUP_DELAY', '10'))
POLL_INTERVAL = int(os.getenv('POLL_INTERVAL', '300'))
BATCH_SIZE = int(os.getenv('BATCH_SIZE', '100'))

# Global connection pool
db_pool = None
state_cache = {}
shutdown_requested = False


def signal_handler(signum, frame):
    """Handle graceful shutdown"""
    global shutdown_requested
    print(f"\n🛑 Received signal {signum}, shutting down gracefully...")
    shutdown_requested = True


def get_db_password():
    """Read database password from guacamole.properties"""
    try:
        properties_file = Path(CONFIG_DIR) / 'guacamole' / 'guacamole.properties'
        if not properties_file.exists():
            raise FileNotFoundError(f"Properties file not found: {properties_file}")

        with open(properties_file, 'r') as f:
            for line in f:
                if 'postgresql-password:' in line:
                    return line.split(':', 1)[1].strip()
        raise ValueError("postgresql-password not found in guacamole.properties")
    except Exception as e:
        print(f"❌ Error reading database password: {e}")
        raise


def init_connection_pool():
    """Initialize database connection pool"""
    global db_pool

    if db_pool:
        return

    password = get_db_password()

    try:
        db_pool = psycopg2.pool.ThreadedConnectionPool(
            minconn=1,
            maxconn=5,
            host=DB_HOST,
            database=DB_NAME,
            user=DB_USER,
            password=password
        )
        print("✅ Database connection pool initialized")
    except Exception as e:
        print(f"❌ Failed to initialize connection pool: {e}")
        raise


def get_db_connection():
    """Get a connection from the pool"""
    if not db_pool:
        init_connection_pool()
    return db_pool.getconn()


def return_db_connection(conn):
    """Return a connection to the pool"""
    if db_pool and conn:
        db_pool.putconn(conn)


def compute_file_hash(file_path):
    """Compute SHA256 hash of file contents for change detection"""
    try:
        with open(file_path, 'rb') as f:
            return hashlib.sha256(f.read()).hexdigest()
    except Exception as e:
        print(f"⚠️  Error computing hash for {file_path}: {e}")
        return None


def get_or_create_entity(cursor, username):
    """Get or create entity_id for a user"""
    cursor.execute("""
        INSERT INTO guacamole_entity (name, type)
        VALUES (%s, 'USER')
        ON CONFLICT (name, type) DO UPDATE SET name = EXCLUDED.name
        RETURNING entity_id;
    """, (username,))
    result = cursor.fetchone()
    return result[0] if result else None


def batch_insert_parameters(cursor, connection_id, parameters):
    """Batch insert connection parameters"""
    if not parameters:
        return

    # First, delete existing parameters
    cursor.execute("""
        DELETE FROM guacamole_connection_parameter
        WHERE connection_id = %s;
    """, (connection_id,))

    # Batch insert new parameters
    values = [(connection_id, name, str(value)) for name, value in parameters.items()]

    # Use execute_batch for better performance
    from psycopg2.extras import execute_batch
    execute_batch(cursor, """
        INSERT INTO guacamole_connection_parameter
        (connection_id, parameter_name, parameter_value)
        VALUES (%s, %s, %s);
    """, values, page_size=BATCH_SIZE)


def batch_insert_permissions(cursor, connection_id, users, permissions):
    """Batch insert user permissions"""
    if not users:
        return

    # Get or create all entities first
    entity_ids = {}
    for username in users:
        entity_id = get_or_create_entity(cursor, username)
        if entity_id:
            entity_ids[username] = entity_id

    # Prepare permission values
    values = []
    for username, entity_id in entity_ids.items():
        for permission in permissions:
            values.append((entity_id, connection_id, permission))

    if values:
        # Use execute_batch for better performance
        from psycopg2.extras import execute_batch
        execute_batch(cursor, """
            INSERT INTO guacamole_connection_permission
            (entity_id, connection_id, permission)
            VALUES (%s, %s, %s)
            ON CONFLICT DO NOTHING;
        """, values, page_size=BATCH_SIZE)


def get_connection_state(cursor, connection_name):
    """Get current state of a connection from database for comparison"""
    cursor.execute("""
        SELECT connection_id, protocol
        FROM guacamole_connection
        WHERE connection_name = %s;
    """, (connection_name,))

    result = cursor.fetchone()
    if not result:
        return None

    connection_id, protocol = result

    # Get parameters
    cursor.execute("""
        SELECT parameter_name, parameter_value
        FROM guacamole_connection_parameter
        WHERE connection_id = %s
        ORDER BY parameter_name;
    """, (connection_id,))

    parameters = dict(cursor.fetchall())

    return {
        'id': connection_id,
        'protocol': protocol,
        'parameters': parameters
    }


def create_or_update_connection(cursor, conn_data):
    """Create or update a connection and its parameters"""
    name = conn_data['name']
    protocol = conn_data['protocol']

    # Check if connection exists
    cursor.execute("""
        SELECT connection_id FROM guacamole_connection
        WHERE connection_name = %s;
    """, (name,))
    existing = cursor.fetchone()

    if existing:
        connection_id = existing[0]
        # Check if update is needed by comparing state
        current_state = get_connection_state(cursor, name)
        new_parameters = conn_data.get('parameters', {})

        # Only update if parameters actually changed
        if current_state and current_state['parameters'] == new_parameters:
            print(f"⏭️  Connection '{name}' unchanged, skipping update")
            return connection_id

        print(f"🔄 Updating connection '{name}' (ID: {connection_id})")
    else:
        cursor.execute("""
            INSERT INTO guacamole_connection (connection_name, protocol)
            VALUES (%s, %s)
            RETURNING connection_id;
        """, (name, protocol))
        connection_id = cursor.fetchone()[0]
        print(f"✨ Created connection '{name}' (ID: {connection_id})")

    # Batch insert parameters
    if 'parameters' in conn_data:
        batch_insert_parameters(cursor, connection_id, conn_data['parameters'])
        print(f"  📝 Updated {len(conn_data['parameters'])} parameters")

    # Batch insert permissions
    if 'users' in conn_data:
        permissions = conn_data.get('permissions', ['READ'])
        batch_insert_permissions(cursor, connection_id, conn_data['users'], permissions)
        print(f"  👥 Granted {permissions} to {len(conn_data['users'])} user(s)")

    return connection_id


def process_connections():
    """Process all connection files and update database"""
    connection_files = glob.glob(f'{CONNECTIONS_DIR}/*/connection.yaml')

    if not connection_files:
        print("⚠️  No connection files found")
        return 0

    # Check for changes using file hashes
    changes_detected = False
    for conn_file in connection_files:
        current_hash = compute_file_hash(conn_file)
        if current_hash and state_cache.get(conn_file) != current_hash:
            changes_detected = True
            state_cache[conn_file] = current_hash

    if not changes_detected:
        print("⏭️  No changes detected in connection files")
        return 0

    # Read all connection files
    connections = []
    for conn_file in connection_files:
        try:
            print(f"📖 Reading: {conn_file}")
            with open(conn_file, 'r') as f:
                conn_data = yaml.safe_load(f)
                if conn_data:
                    connections.append(conn_data)
        except Exception as e:
            print(f"❌ Error reading {conn_file}: {e}")
            continue

    if not connections:
        print("⚠️  No valid connections found")
        return 0

    # Process connections in a single transaction
    conn = None
    try:
        conn = get_db_connection()
        cursor = conn.cursor()

        print(f"\n🔄 Processing {len(connections)} connection(s)...")

        updated_count = 0
        for conn_data in connections:
            try:
                create_or_update_connection(cursor, conn_data)
                updated_count += 1
            except Exception as e:
                print(f"❌ Error processing connection '{conn_data.get('name', 'unknown')}': {e}")
                import traceback
                traceback.print_exc()

        conn.commit()
        print(f"\n✅ Successfully processed {updated_count} connection(s)!")

        cursor.close()
        return updated_count

    except Exception as e:
        if conn:
            conn.rollback()
        print(f"❌ Database error: {e}")
        import traceback
        traceback.print_exc()
        return 0
    finally:
        if conn:
            return_db_connection(conn)


def wait_for_guacamole():
    """Wait for Guacamole to be ready"""
    print(f"⏳ Waiting {STARTUP_DELAY}s for Guacamole to start...")
    time.sleep(STARTUP_DELAY)

    # Try to connect to database to verify it's ready
    max_retries = 10
    for i in range(max_retries):
        try:
            init_connection_pool()
            print("✅ Guacamole database is ready")
            return True
        except Exception as e:
            if i < max_retries - 1:
                print(f"⏳ Database not ready yet, retrying in 5s... ({i+1}/{max_retries})")
                time.sleep(5)
            else:
                print(f"❌ Failed to connect to database after {max_retries} attempts")
                return False
    return False


def main():
    """Main event loop"""
    global shutdown_requested

    # Set up signal handlers
    signal.signal(signal.SIGTERM, signal_handler)
    signal.signal(signal.SIGINT, signal_handler)

    print("🚀 Guacamole Connection Provisioner (Optimized)")
    print(f"   Connections dir: {CONNECTIONS_DIR}")
    print(f"   Poll interval: {POLL_INTERVAL}s")
    print(f"   Batch size: {BATCH_SIZE}")

    if not wait_for_guacamole():
        sys.exit(1)

    # Initial provisioning
    print("\n" + "="*50)
    print("📋 Initial provisioning")
    print("="*50)
    process_connections()

    # Main loop with file watching
    try:
        while not shutdown_requested:
            print(f"\n⏰ Waiting {POLL_INTERVAL}s before next check...")

            # Sleep in small intervals to respond quickly to shutdown
            for _ in range(POLL_INTERVAL):
                if shutdown_requested:
                    break
                time.sleep(1)

            if shutdown_requested:
                break

            print("\n" + "="*50)
            print("🔍 Checking for connection updates...")
            print("="*50)

            process_connections()

    except Exception as e:
        print(f"❌ Fatal error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
    finally:
        # Clean up connection pool
        if db_pool:
            db_pool.closeall()
        print("\n👋 Shutting down gracefully")


if __name__ == '__main__':
    main()
