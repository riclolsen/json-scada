<template>
  <div class="langPosition">
      <img ref="flag_ar.svg" src="../assets/flag_ar.svg" alt="arabic"     style="display:none" >
      <img ref="flag_de.svg" src="../assets/flag_de.svg" alt="german"     style="display:none" >
      <img ref="flag_en.svg" src="../assets/flag_en.svg" alt="english"    style="display:none" >
      <img ref="flag_es.svg" src="../assets/flag_es.svg" alt="spanish"    style="display:none" >
      <img ref="flag_fa.svg" src="../assets/flag_fa.svg" alt="persian"    style="display:none" >
      <img ref="flag_pt.svg" src="../assets/flag_pt.svg" alt="portuguese" style="display:none" >
      <img ref="flag_ru.svg" src="../assets/flag_ru.svg" alt="russian"    style="display:none" >
      <img ref="flag_uk.svg" src="../assets/flag_uk.svg" alt="ukrainian"  style="display:none" >
      <img ref="flag_zh.svg" src="../assets/flag_zh.svg" alt="chinese"    style="display:none" >
    <button
      href="#"
      class="flex items-center langDropdownButton"
      @click="toggleVisibility"
      @keydown.space.exact.prevent="toggleVisibility"
      @keydown.esc.exact="hideDropdown"
      @keydown.shift.tab="hideDropdown"
      @keydown.up.exact.prevent="startArrowKeys"
      @keydown.down.exact.prevent="startArrowKeys"
    >
      <img ref="currentFlag" alt="flag" class="flagShadow" :key="componentKey" />
    </button>
    <transition name="dropdown-fade">
      <ul
        v-if="isVisible"
        ref="dropdown"
        v-on-clickaway="hideDropdown"
        class="langBox"
      >
        <li v-for="(lang, idx) in langs" :key="idx">
          <a
            href="#"
            class="flex langHover"
            @click.prevent="setLocale(lang.shortName)"
            @keydown.up.exact.prevent
            @keydown.tab.exact="focusNext(false)"
            @keydown.down.exact.prevent="focusNext(true)"
            @keydown.esc.exact="hideDropdown"
          >
            <img :src="$refs[lang.flag].src" :alt="lang.longName" class="flag" />
            <span class="localLang">{{ lang.localName }}</span>
          </a>
        </li>
      </ul>
    </transition>
  </div>
</template>

<script>
export default {
  name: "LanguageSwitcher",
  data() {
    return {
      isVisible: false,
      focusedIndex: 0,
      componentKey: 0,
      langs: [
        {
          shortName: "en",
          longName: "English",
          localName: "English",
          flag: "flag_en.svg"
        },
        {
          shortName: "pt",
          longName: "Portuguese",
          localName: "Português",
          flag: "flag_pt.svg"
        },
        {
          shortName: "fa",
          longName: "Persian",
          localName: "Persian",
          flag: "flag_fa.svg"
        },
        {
          shortName: "ar",
          longName: "Arabian",
          localName: "Arabian",
          flag: "flag_ar.svg"
        },
        {
          shortName: "uk",
          longName: "Ukranian",
          localName: "Ukranian",
          flag: "flag_uk.svg"
        },
        {
          shortName: "es",
          longName: "Spanish",
          localName: "Español",
          flag: "flag_es.svg"
        },
        {
          shortName: "ru",
          longName: "Russian",
          localName: "русский",
          flag: "flag_ru.svg"
        },
        {
          shortName: "de",
          longName: "German",
          localName: "Deutsch",
          flag: "flag_de.svg"
        },
        {
          shortName: "zh",
          longName: "Chinese",
          localName: "中文",
          flag: "flag_zh.svg"
        }
      ]
    };
  },
  mounted() {
     this.$refs.currentFlag.src = this.$refs[`flag_${this.$i18n.locale}.svg`].src;
  },
  methods: {
    toggleVisibility() {
      this.isVisible = !this.isVisible;
    },
    hideDropdown() {
      this.isVisible = false;
      this.focusedIndex = 0;
    },
    startArrowKeys() {
      if (this.isVisible) {
        this.$refs.dropdown.children[0].children[0].focus();
      }
    },
    focusPrevious(isArrowKey) {
      this.focusedIndex = this.focusedIndex - 1;
      if (isArrowKey) {
        this.focusItem();
      }
    },
    focusNext(isArrowKey) {
      this.focusedIndex = this.focusedIndex + 1;
      if (isArrowKey) {
        this.focusItem();
      }
    },
    focusItem() {
      this.$refs.dropdown.children[this.focusedIndex].children[0].focus();
    },
    setLocale(locale) {
      this.$i18n.locale = locale;
      localStorage.setItem("lang", locale);
      this.$refs.currentFlag.src = this.$refs[`flag_${this.$i18n.locale}.svg`].src;

      // Uncomment the below lines if you need language prefixes to be shown in the URL
      // example: en.website.com (additional router configuration required)

      // this.$router.push({
      //   params: { lang: locale }
      // })
      this.hideDropdown();
    }
  }
};
</script>

<style lang="scss" scoped>
// Position of language dot
.langPosition {
  z-index: 99999999;
  position: absolute;
  display: flex;
  justify-content: center;
  top: 5px;
  right: 5px;
}

// Box that holds all language options
.langBox {
  user-select: none;
  padding: 5px 5px 5px -5px;
  width: max-content;
  box-shadow: 0px 3px 6px rgb(204, 210, 235);
}

// Language name text
.localLang {
  position: absolute;
  margin-top: 3px;
  font-size: 18px;
  padding-left: 15px;
  font-weight: 400;
  vertical-align: middle;
}

// Animations
.dropdown-fade-enter-active,
.dropdown-fade-leave-active {
  transition: all 0.1s ease-in-out;
}
.dropdown-fade-enter,
.dropdown-fade-leave-to {
  opacity: 0;
  transform: translateY(-12px);
}

.langDropdownButton {
  outline: none;
  cursor: pointer !important;
  background-color: transparent !important;
  border: none;
  padding-right: 0px;
}

.shortName {
  margin-top: 5px !important;
}

ul {
  text-align: left;

  min-width: 200px;
  border-radius: 8px;
  background: white;
  z-index: 500;
  position: absolute;
  list-style: none;
  padding: 10px 10px !important;
  margin: 0px !important;
  top: 0px;
  right: 0px;
  li {
    padding: 5px 0;
  }
}

.flag {
  width: 26px;
}

.flagShadow {
  border-radius: 50px;
  box-shadow: 0px 3px 3px rgb(204, 210, 235);
  width: 35px;
}

.langHover {
  margin: 10px;
  text-decoration: none;
  color: #222;
  transition: all 0.1s ease-in-out;
  &:hover {
    color: #007aff;
  }
}
</style>
