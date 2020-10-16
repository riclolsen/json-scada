<template>
    <v-app>
        <app-navigation></app-navigation>

        <v-main transition="slide-x-transition">
            <tabbed-options />

            <!--
            <router-view></router-view>
            -->
        </v-main>
    </v-app>
</template>

<script>
import AppNavigation from '@/components/AppNavigation';
import TabbedOptions from '@/components/TabbedOptions';

export default {
    name: 'App',
    components: {
        AppNavigation,
        TabbedOptions
    }
};
</script>

<style>
</style>