import { Component, inject, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import {
  SignerMetaData_VaultSnapshot,
  StartupService,
  StorageService,
} from '@common';
import { getNewStorageServiceConfig } from '../../../common/data/get-new-storage-service-config';
import browser from 'webextension-polyfill';

@Component({
  selector: 'app-vault-create-home',
  imports: [FormsModule],
  templateUrl: './home.component.html',
  styleUrl: './home.component.scss',
})
export class HomeComponent implements OnInit {
  readonly router = inject(Router);
  readonly #storage = inject(StorageService);
  readonly #startup = inject(StartupService);

  snapshots: SignerMetaData_VaultSnapshot[] = [];
  selectedSnapshot: SignerMetaData_VaultSnapshot | undefined;

  ngOnInit(): void {
    this.#loadData();
  }

  async #loadData() {
    const data = (await browser.storage.local.get('vaultSnapshots')) as {
      vaultSnapshots?: SignerMetaData_VaultSnapshot[];
    };
    this.snapshots = (data.vaultSnapshots ?? []).sortBy((x) => x.fileName, 'desc');
    this.selectedSnapshot = this.snapshots[0];
  }

  async onClickImport() {
    if (!this.selectedSnapshot) return;
    try {
      await this.#storage.deleteVault(true);
      await this.#storage.importVault(this.selectedSnapshot.data);
      this.#storage.isInitialized = false;
      this.#startup.startOver(getNewStorageServiceConfig());
    } catch (error) {
      console.log(error);
    }
  }

  openOptionsPage() {
    browser.runtime.openOptionsPage();
  }
}
