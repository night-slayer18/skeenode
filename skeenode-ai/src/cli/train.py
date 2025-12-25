#!/usr/bin/env python3
"""
Training CLI

Command-line interface for model training.

Usage:
    python -m src.cli.train --database-url postgresql://... --activate
"""

import argparse
import logging
import sys

from ..data.training import run_training


def main():
    parser = argparse.ArgumentParser(
        description="Train failure prediction model from historical data"
    )
    parser.add_argument(
        "--database-url",
        required=True,
        help="PostgreSQL connection string",
    )
    parser.add_argument(
        "--lookback-days",
        type=int,
        default=90,
        help="Days of history to use (default: 90)",
    )
    parser.add_argument(
        "--activate",
        action="store_true",
        help="Activate model on successful training",
    )
    parser.add_argument(
        "--min-accuracy",
        type=float,
        default=0.7,
        help="Minimum accuracy to register model (default: 0.7)",
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Verbose logging",
    )
    
    args = parser.parse_args()
    
    # Setup logging
    level = logging.DEBUG if args.verbose else logging.INFO
    logging.basicConfig(
        level=level,
        format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    )
    
    print("=" * 60)
    print("Skeenode AI - Model Training")
    print("=" * 60)
    print(f"Database: {args.database_url[:50]}...")
    print(f"Lookback: {args.lookback_days} days")
    print(f"Auto-activate: {args.activate}")
    print(f"Min accuracy: {args.min_accuracy}")
    print("=" * 60)
    
    # Run training
    result = run_training(
        database_url=args.database_url,
        activate=args.activate,
    )
    
    # Print results
    print("\n" + "=" * 60)
    print("TRAINING RESULTS")
    print("=" * 60)
    
    if result.success:
        print(f"✅ SUCCESS")
        print(f"   Version: {result.version_id}")
        print(f"   Samples: {result.samples_used}")
        print(f"   Time: {result.training_time_seconds:.1f}s")
        print("\nMetrics:")
        for name, value in result.metrics.items():
            print(f"   {name}: {value:.4f}")
        sys.exit(0)
    else:
        print(f"❌ FAILED")
        print(f"   Error: {result.error}")
        sys.exit(1)


if __name__ == "__main__":
    main()
